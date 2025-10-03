package exec

import (
	"archive/tar"
	"bytes"
	"context"
	// "io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type SandboxLimits struct {
	WallTime time.Duration
	MemoryB  int64
	NanoCPUs int64
}

type Sandbox struct {
	cli    *client.Client
	image  string
	limits SandboxLimits
}

func NewSandbox(image string, limits SandboxLimits) (*Sandbox, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Sandbox{cli: cli, image: image, limits: limits}, nil
}

func (s *Sandbox) Run(ctx context.Context, fileName string, code []byte, cmds [][]string,
	onStdout func([]byte), onStderr func([]byte)) (exit int, timedOut bool, err error) {

	ctx, cancel := context.WithTimeout(ctx, s.limits.WallTime)
	defer cancel()

	hostCfg := &container.HostConfig{
		NetworkMode:    "none",
		ReadonlyRootfs: true,
		Mounts: []mount.Mount{
			{Type: mount.TypeTmpfs, Target: "/tmp"},
			{Type: mount.TypeTmpfs, Target: "/workspace"},
		},
		Resources: container.Resources{
			Memory:   s.limits.MemoryB,
			NanoCPUs: s.limits.NanoCPUs,
			// Optional: PidsLimit: 128, Ulimits: []*units.ulimits ...
		},
		SecurityOpt: []string{"no-new-privileges"},
	}

	conf := &container.Config{
		Image:        s.image,
		Cmd:          []string{"bash", "-lc", "sleep infinity"},
		Tty:          false,
		AttachStdout: false,
		AttachStderr: false,
		WorkingDir:   "/workspace",
	}

	create, err := s.cli.ContainerCreate(ctx, conf, hostCfg, nil, nil, "")
	if err != nil {
		return 0, false, err
	}
	cid := create.ID
	defer func() {
		_ = s.cli.ContainerRemove(context.Background(), cid, types.ContainerRemoveOptions{Force: true})
	}()

	if err := s.cli.ContainerStart(ctx, cid, types.ContainerStartOptions{}); err != nil {
		return 0, false, err
	}

	// Copy code file
	if err := s.copyFile(ctx, cid, "/workspace/"+fileName, code, 0600); err != nil {
		_ = s.cli.ContainerKill(context.Background(), cid, "SIGKILL")
		return 0, false, err
	}

	// Run steps (compile -> run)
	for i, cmd := range cmds {
		execID, attachCloser, err := s.execStart(ctx, cid, cmd)
		if err != nil {
			_ = s.cli.ContainerKill(context.Background(), cid, "SIGKILL")
			return 0, false, err
		}

		// Demux multiplexed docker stream
		_, _ = stdcopy.StdCopy(
			writerFunc(onStdout),
			writerFunc(onStderr),
			attachCloser.Reader,
		)
		attachCloser.Close()

		ir, ierr := s.cli.ContainerExecInspect(ctx, execID)
		if ierr != nil {
			_ = s.cli.ContainerKill(context.Background(), cid, "SIGKILL")
			return 0, false, ierr
		}

		if ir.ExitCode != 0 {
			return ir.ExitCode, false, nil
		}
		if i == len(cmds)-1 {
			return 0, false, nil
		}
	}
	return 0, false, nil
}

func (s *Sandbox) execStart(ctx context.Context, containerID string, cmd []string) (execID string, attach types.HijackedResponse, err error) {
	execResp, err := s.cli.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          cmd,
		WorkingDir:   "/workspace",
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	})
	if err != nil {
		return "", types.HijackedResponse{}, err
	}
	attach, err = s.cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{Tty: false})
	if err != nil {
		return "", types.HijackedResponse{}, err
	}
	if err := s.cli.ContainerExecStart(ctx, execResp.ID, types.ExecStartCheck{Tty: false}); err != nil {
		attach.Close()
		return "", types.HijackedResponse{}, err
	}
	return execResp.ID, attach, nil
}

func (s *Sandbox) copyFile(ctx context.Context, cid, absPath string, content []byte, mode int64) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: absPath[1:],
		Mode: mode,
		Size: int64(len(content)),
	}); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return s.cli.CopyToContainer(ctx, cid, "/", &buf, types.CopyToContainerOptions{})
}

type writerFunc func([]byte)

func (f writerFunc) Write(p []byte) (int, error) {
	f(p)
	return len(p), nil
}

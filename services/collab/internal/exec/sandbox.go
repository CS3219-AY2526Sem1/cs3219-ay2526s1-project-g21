package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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

var ErrDockerUnavailable = errors.New("docker daemon unreachable")

func NewSandbox(image string, limits SandboxLimits) (*Sandbox, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, translateDockerErr(err)
	}
	return &Sandbox{cli: cli, image: image, limits: limits}, nil
}

func (s *Sandbox) Run(ctx context.Context, fileName string, code []byte, cmds [][]string,
	onStdout func([]byte), onStderr func([]byte)) (exit int, timedOut bool, err error) {

	ctx, cancel := context.WithTimeout(ctx, s.limits.WallTime)
	defer cancel()

	if err := s.ensureImage(ctx); err != nil {
		return -1, false, translateDockerErr(err)
	}

	hostCfg := &container.HostConfig{
		NetworkMode:    "none",
		ReadonlyRootfs: false,
		Resources: container.Resources{
			Memory:   s.limits.MemoryB,
			NanoCPUs: s.limits.NanoCPUs,
			// Optional: PidsLimit: 128, Ulimits: []*units.ulimits ...
		},
		SecurityOpt: []string{"no-new-privileges"},
	}

	conf := &container.Config{
		Image:        s.image,
		Cmd:          []string{"/bin/sh", "-c", "sleep infinity"},
		Tty:          false,
		AttachStdout: false,
		AttachStderr: false,
		WorkingDir:   "/workspace",
		Env:          []string{"PYTHONDONTWRITEBYTECODE=1"},
	}

	create, err := s.cli.ContainerCreate(ctx, conf, hostCfg, nil, nil, "")
	if err != nil {
		return -1, false, translateDockerErr(err)
	}
	cid := create.ID
	defer func() {
		_ = s.cli.ContainerRemove(context.Background(), cid, types.ContainerRemoveOptions{Force: true})
	}()

	if err := s.cli.ContainerStart(ctx, cid, types.ContainerStartOptions{}); err != nil {
		return -1, false, translateDockerErr(err)
	}

	// Copy code file
	if err := s.copyFile(ctx, cid, "/workspace/"+fileName, code, 0600); err != nil {
		_ = s.cli.ContainerKill(context.Background(), cid, "SIGKILL")
		return -1, false, translateDockerErr(err)
	}

	// Run steps (compile -> run)
	for i, cmd := range cmds {
		execID, attachCloser, err := s.execStart(ctx, cid, cmd)
		if err != nil {
			_ = s.cli.ContainerKill(context.Background(), cid, "SIGKILL")
			return -1, false, translateDockerErr(err)
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
			return -1, false, translateDockerErr(ierr)
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

func (s *Sandbox) ensureImage(ctx context.Context) error {
	_, _, err := s.cli.ImageInspectWithRaw(ctx, s.image)
	if err == nil {
		return nil
	}
	if client.IsErrNotFound(err) {
		pullCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		reader, pullErr := s.cli.ImagePull(pullCtx, s.image, types.ImagePullOptions{})
		if pullErr != nil {
			return translateDockerErr(pullErr)
		}
		defer reader.Close()
		_, _ = io.Copy(io.Discard, reader)
		return nil
	}
	return translateDockerErr(err)
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
		return "", types.HijackedResponse{}, translateDockerErr(err)
	}
	attach, err = s.cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{Tty: false})
	if err != nil {
		return "", types.HijackedResponse{}, translateDockerErr(err)
	}
	if err := s.cli.ContainerExecStart(ctx, execResp.ID, types.ExecStartCheck{Tty: false}); err != nil {
		attach.Close()
		return "", types.HijackedResponse{}, translateDockerErr(err)
	}
	return execResp.ID, attach, nil
}

func (s *Sandbox) copyFile(ctx context.Context, cid, absPath string, content []byte, mode int64) error {
	if absPath == "" || !strings.HasPrefix(absPath, "/") {
		return fmt.Errorf("invalid path %q", absPath)
	}
	dir := path.Dir(absPath)
	if err := s.runCommand(ctx, cid, fmt.Sprintf("mkdir -p %s", shellQuote(dir))); err != nil {
		return err
	}
	writeCmd := fmt.Sprintf("cat > %s", shellQuote(absPath))
	if err := s.execWithInput(ctx, cid, writeCmd, content); err != nil {
		return err
	}
	return s.runCommand(ctx, cid, fmt.Sprintf("chmod %o %s", mode&0o777, shellQuote(absPath)))
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (s *Sandbox) runCommand(ctx context.Context, cid, cmd string) error {
	execID, attach, err := s.execStart(ctx, cid, []string{"/bin/sh", "-c", cmd})
	if err != nil {
		return err
	}
	_, _ = stdcopy.StdCopy(io.Discard, io.Discard, attach.Reader)
	attach.Close()
	inspect, err := s.cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		return translateDockerErr(err)
	}
	if inspect.ExitCode != 0 {
		return fmt.Errorf("command failed (%s) exit=%d", cmd, inspect.ExitCode)
	}
	return nil
}

func translateDockerErr(err error) error {
	if err == nil {
		return nil
	}
	if client.IsErrConnectionFailed(err) {
		return ErrDockerUnavailable
	}
	return err
}

func (s *Sandbox) execWithInput(ctx context.Context, cid, command string, payload []byte) error {
	execResp, err := s.cli.ContainerExecCreate(ctx, cid, types.ExecConfig{
		Cmd:          []string{"/bin/sh", "-c", command},
		WorkingDir:   "/workspace",
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          false,
	})
	if err != nil {
		return err
	}
	attach, err := s.cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{Tty: false})
	if err != nil {
		return err
	}
	defer attach.Close()
	if err := s.cli.ContainerExecStart(ctx, execResp.ID, types.ExecStartCheck{Tty: false}); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := attach.Conn.Write(payload); err != nil {
			return err
		}
	}
	if closer, ok := attach.Conn.(interface{ CloseWrite() error }); ok {
		_ = closer.CloseWrite()
	}
	_, _ = stdcopy.StdCopy(io.Discard, io.Discard, attach.Reader)
	inspect, err := s.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return err
	}
	if inspect.ExitCode != 0 {
		return fmt.Errorf("write failed (%s) exit=%d", command, inspect.ExitCode)
	}
	return nil
}

type writerFunc func([]byte)

func (f writerFunc) Write(p []byte) (int, error) {
	f(p)
	return len(p), nil
}

package runtime

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
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type Language string

const (
	LangPython Language = "python"
	LangJava   Language = "java"
	LangCPP    Language = "cpp"
)

type LanguageSpec struct {
	FileName   string
	RunCmd     []string
	CompileCmd []string
	ExecCmd    []string
}

type Limits struct {
	WallTime time.Duration
	MemoryB  int64
	NanoCPUs int64
}

type ExitInfo struct {
	Code     int  `json:"code"`
	TimedOut bool `json:"timedOut"`
}

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Result struct {
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
	Exit   ExitInfo `json:"exit"`
	Events []Event  `json:"events"`
	Error  string   `json:"error,omitempty"`
}

type dockerClient interface {
	ImageInspectWithRaw(ctx context.Context, image string) (types.ImageInspect, []byte, error)
	ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerKill(ctx context.Context, containerID string, signal string) error
	ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, config types.ExecStartCheck) (types.HijackedResponse, error)
	ContainerExecStart(ctx context.Context, execID string, config types.ExecStartCheck) error
	ContainerExecInspect(ctx context.Context, execID string) (types.ContainerExecInspect, error)
}

type Sandbox struct {
	cli    dockerClient
	image  string
	limits Limits
}

var newDockerClient = func() (dockerClient, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

var ErrDockerUnavailable = errors.New("docker daemon unreachable")

func NewSandbox(image string, limits Limits) (*Sandbox, error) {
	cli, err := newDockerClient()
	if err != nil {
		return nil, translateDockerErr(err)
	}
	if limits.WallTime <= 0 {
		limits.WallTime = 10 * time.Second
	}
	if limits.MemoryB == 0 {
		limits.MemoryB = 512 * 1024 * 1024
	}
	if limits.NanoCPUs == 0 {
		limits.NanoCPUs = 1_000_000_000
	}
	return &Sandbox{cli: cli, image: image, limits: limits}, nil
}

func Execute(ctx context.Context, lang Language, code string, limits Limits) (Result, error) {
	_, image, fileName, cmds, err := langSpec(lang)
	if err != nil {
		return Result{}, err
	}

	sbx, err := NewSandbox(image, limits)
	if err != nil {
		msg := mapSandboxError(err)
		res := Result{Error: msg, Exit: ExitInfo{Code: -1, TimedOut: false}}
		res.Events = append(res.Events, Event{Type: "error", Data: msg})
		res.Events = append(res.Events, Event{Type: "exit", Data: res.Exit})
		return res, nil
	}

	runCtx, cancel := context.WithTimeout(ctx, sbx.limits.WallTime)
	defer cancel()

	var stdoutBuf, stderrBuf strings.Builder
	result := Result{Events: make([]Event, 0, len(cmds)*2+1)}

	exit, timedOut, runErr := sbx.Run(
		runCtx,
		fileName,
		[]byte(code),
		cmds,
		func(p []byte) {
			chunk := string(p)
			stdoutBuf.WriteString(chunk)
			result.Events = append(result.Events, Event{Type: "stdout", Data: chunk})
		},
		func(p []byte) {
			chunk := string(p)
			stderrBuf.WriteString(chunk)
			result.Events = append(result.Events, Event{Type: "stderr", Data: chunk})
		},
	)

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	result.Exit = ExitInfo{Code: exit, TimedOut: timedOut}
	result.Events = append(result.Events, Event{Type: "exit", Data: result.Exit})

	if runErr != nil {
		msg := mapSandboxError(runErr)
		result.Error = msg
		result.Events = append(result.Events, Event{Type: "error", Data: msg})
	}

	return result, nil
}

func (s *Sandbox) Run(ctx context.Context, fileName string, code []byte, cmds [][]string,
	onStdout func([]byte), onStderr func([]byte)) (exit int, timedOut bool, err error) {

	if err := s.ensureImage(ctx); err != nil {
		return -1, false, translateDockerErr(err)
	}

	hostCfg := &container.HostConfig{
		NetworkMode:    "none",
		ReadonlyRootfs: false,
		Resources: container.Resources{
			Memory:   s.limits.MemoryB,
			NanoCPUs: s.limits.NanoCPUs,
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

	if err := s.copyFile(ctx, cid, "/workspace/"+fileName, code, 0600); err != nil {
		_ = s.cli.ContainerKill(context.Background(), cid, "SIGKILL")
		return -1, false, translateDockerErr(err)
	}

	for i, cmd := range cmds {
		execID, attachCloser, err := s.execStart(ctx, cid, cmd)
		if err != nil {
			_ = s.cli.ContainerKill(context.Background(), cid, "SIGKILL")
			return -1, false, translateDockerErr(err)
		}

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

func translateDockerErr(err error) error {
	if err == nil {
		return nil
	}
	if client.IsErrConnectionFailed(err) {
		return ErrDockerUnavailable
	}
	return err
}

func langSpec(lang Language) (LanguageSpec, string, string, [][]string, error) {
	switch lang {
	case LangPython:
		return LanguageSpec{
				FileName: "main.py",
				RunCmd:   []string{"python3", "main.py"},
			},
			"python:3.11-slim",
			"main.py",
			[][]string{{"python3", "main.py"}},
			nil

	case LangJava:
		return LanguageSpec{
				FileName:   "Main.java",
				CompileCmd: []string{"javac", "Main.java"},
				ExecCmd:    []string{"/bin/sh", "-c", "java Main"},
			},
			"eclipse-temurin:17-jdk",
			"Main.java",
			[][]string{{"javac", "Main.java"}, {"/bin/sh", "-c", "java Main"}},
			nil

	case LangCPP:
		return LanguageSpec{
				FileName:   "main.cpp",
				CompileCmd: []string{"g++", "-O2", "-std=c++17", "main.cpp", "-o", "main"},
				ExecCmd:    []string{"./main"},
			},
			"gcc:13",
			"main.cpp",
			[][]string{{"g++", "-O2", "-std=c++17", "main.cpp", "-o", "main"}, {"./main"}},
			nil
	default:
		return LanguageSpec{}, "", "", nil, errors.New("unsupported_language")
	}
}

func mapSandboxError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrDockerUnavailable) {
		return "sandbox_unavailable"
	}
	if err.Error() == "unsupported_language" {
		return "unsupported_language"
	}
	return "sandbox_error"
}

func WarmImages(ctx context.Context, langs ...Language) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(langs) == 0 {
		langs = []Language{LangPython, LangJava, LangCPP}
	}
	for _, lang := range langs {
		if err := warmImage(ctx, lang); err != nil {
			return fmt.Errorf("warm %s: %w", lang, err)
		}
	}
	return nil
}

func warmImage(ctx context.Context, lang Language) error {
	_, image, _, _, err := langSpec(lang)
	if err != nil {
		return err
	}
	sbx, err := NewSandbox(image, Limits{})
	if err != nil {
		return err
	}
	if closer, ok := sbx.cli.(interface{ Close() error }); ok {
		defer closer.Close()
	}
	return sbx.ensureImage(ctx)
}

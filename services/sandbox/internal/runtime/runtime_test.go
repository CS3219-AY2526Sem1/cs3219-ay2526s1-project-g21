package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestLangSpec(t *testing.T) {
	spec, image, fileName, cmds, err := langSpec(LangPython)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if spec.FileName != "main.py" || image != "python:3.11-slim" || fileName != "main.py" {
		t.Fatalf("unexpected python spec: %+v image=%s file=%s", spec, image, fileName)
	}
	if len(cmds) != 1 || !reflect.DeepEqual(cmds[0], []string{"python3", "main.py"}) {
		t.Fatalf("unexpected python commands: %v", cmds)
	}

	spec, image, fileName, cmds, err = langSpec(LangJava)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if spec.FileName != "Main.java" || image != "eclipse-temurin:17-jdk" || fileName != "Main.java" {
		t.Fatalf("unexpected java spec: %+v image=%s file=%s", spec, image, fileName)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected java to have compile+exec commands: %v", cmds)
	}

	spec, image, fileName, cmds, err = langSpec(LangCPP)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if spec.FileName != "main.cpp" || image != "gcc:13" || fileName != "main.cpp" {
		t.Fatalf("unexpected cpp spec: %+v image=%s file=%s", spec, image, fileName)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected cpp to have compile+exec commands: %v", cmds)
	}

	_, _, _, _, err = langSpec(Language("unknown"))
	if err == nil || err.Error() != "unsupported_language" {
		t.Fatalf("expected unsupported_language error, got %v", err)
	}
}

func TestMapSandboxError(t *testing.T) {
	if got := mapSandboxError(nil); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	if got := mapSandboxError(ErrDockerUnavailable); got != "sandbox_unavailable" {
		t.Fatalf("expected sandbox_unavailable, got %q", got)
	}
	if got := mapSandboxError(errors.New("unsupported_language")); got != "unsupported_language" {
		t.Fatalf("expected unsupported_language, got %q", got)
	}
	if got := mapSandboxError(errors.New("anything_else")); got != "sandbox_error" {
		t.Fatalf("expected sandbox_error, got %q", got)
	}
}

func TestTranslateDockerErr(t *testing.T) {
	if translateDockerErr(nil) != nil {
		t.Fatalf("expected nil for nil input")
	}
	err := client.ErrorConnectionFailed("unix:///var/run/docker.sock")
	if !errors.Is(translateDockerErr(err), ErrDockerUnavailable) {
		t.Fatalf("expected ErrDockerUnavailable")
	}
	someErr := errors.New("boom")
	if translateDockerErr(someErr) != someErr {
		t.Fatalf("expected passthrough error")
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"", "''"},
		{"simple", "'simple'"},
		{"has'single", "'has'\\''single'"},
	}
	for _, tc := range tests {
		if got := shellQuote(tc.in); got != tc.out {
			t.Fatalf("shellQuote(%q)=%q want %q", tc.in, got, tc.out)
		}
	}
}

func TestWriterFunc(t *testing.T) {
	var got []byte
	w := writerFunc(func(p []byte) {
		got = append(got, p...)
	})
	if n, err := w.Write([]byte("hello")); err != nil || n != 5 {
		t.Fatalf("unexpected write result n=%d err=%v", n, err)
	}
	if string(got) != "hello" {
		t.Fatalf("expected callback to capture bytes")
	}
}

func TestCopyFileInvalidPath(t *testing.T) {
	sbx := &Sandbox{}
	if err := sbx.copyFile(context.Background(), "cid", "relative/path", nil, 0); err == nil {
		t.Fatalf("expected error for invalid path")
	}
}

func TestCopyFileExecWithInputError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > '/workspace/main.py'"},
				startErr:  errors.New("start failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	catCall := client.execQueue[1]
	err := sbx.copyFile(context.Background(), "cid", "/workspace/main.py", []byte("code"), 0600)
	if !errors.Is(err, catCall.startErr) {
		t.Fatalf("expected execWithInput error, got %v", err)
	}
}

func TestEnsureImageHappyPath(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	if err := sbx.ensureImage(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.imagePulled {
		t.Fatalf("did not expect pull when inspect succeeds")
	}
}

func TestEnsureImagePullsWhenMissing(t *testing.T) {
	client := &fakeDockerClient{
		t:               t,
		imageInspectErr: errdefs.NotFound(errors.New("missing")),
		createResp:      container.ContainerCreateCreatedBody{ID: "cid"},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	if err := sbx.ensureImage(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !client.imagePulled {
		t.Fatalf("expected image pull when inspect reports not found")
	}
}

func TestEnsureImagePropagatesErrors(t *testing.T) {
	notFound := errors.New("other")
	client := &fakeDockerClient{
		t:               t,
		imageInspectErr: notFound,
		createResp:      container.ContainerCreateCreatedBody{ID: "cid"},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	if err := sbx.ensureImage(context.Background()); !errors.Is(err, notFound) {
		t.Fatalf("expected propagate error, got %v", err)
	}

	client = &fakeDockerClient{
		t:               t,
		imageInspectErr: errdefs.NotFound(errors.New("missing")),
		imagePullErr:    errors.New("pull failed"),
		createResp:      container.ContainerCreateCreatedBody{ID: "cid"},
	}
	sbx = &Sandbox{cli: client, image: "image", limits: Limits{}}
	if err := sbx.ensureImage(context.Background()); !errors.Is(err, client.imagePullErr) {
		t.Fatalf("expected pull error, got %v", err)
	}
}

func TestRunCommandHandlesErrors(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "fail"},
				inspect:   types.ContainerExecInspect{ExitCode: 5},
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	err := sbx.runCommand(context.Background(), "cid", "fail")
	if err == nil || !strings.Contains(err.Error(), "exit=5") {
		t.Fatalf("expected exit code error, got %v", err)
	}
}

func TestRunCommandExecStartError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "oops"},
				createErr: errors.New("exec create failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	call := client.execQueue[0]
	err := sbx.runCommand(context.Background(), "cid", "oops")
	if !errors.Is(err, call.createErr) {
		t.Fatalf("expected exec start error, got %v", err)
	}
}

func TestRunCommandInspectError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd:  []string{"/bin/sh", "-c", "check"},
				inspectErr: client.ErrorConnectionFailed("unix:///var/run/docker.sock"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	err := sbx.runCommand(context.Background(), "cid", "check")
	if !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("expected ErrDockerUnavailable, got %v", err)
	}
}

func TestExecStartCreateError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"noop"},
				createErr: errors.New("create failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	call := client.execQueue[0]
	_, _, err := sbx.execStart(context.Background(), "cid", []string{"noop"})
	if !errors.Is(err, call.createErr) {
		t.Fatalf("expected create error, got %v", err)
	}
}

func TestExecStartAttachError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"noop"},
				attachErr: errors.New("attach failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	call := client.execQueue[0]
	_, _, err := sbx.execStart(context.Background(), "cid", []string{"noop"})
	if !errors.Is(err, call.attachErr) {
		t.Fatalf("expected attach error, got %v", err)
	}
}

func TestExecStartStartErrorClosesAttach(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"noop"},
				startErr:  errors.New("start failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	call := client.execQueue[0]
	_, attach, err := sbx.execStart(context.Background(), "cid", []string{"noop"})
	if !errors.Is(err, call.startErr) {
		t.Fatalf("expected start error, got %v", err)
	}
	if call.conn == nil || !call.conn.closed {
		t.Fatalf("expected attach connection to close on start error")
	}
	if attach.Conn != nil {
		attach.Close()
	}
}

func TestExecWithInputStartError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > file"},
				startErr:  errors.New("start failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	call := client.execQueue[0]
	if err := sbx.execWithInput(context.Background(), "cid", "cat > file", []byte("data")); !errors.Is(err, call.startErr) {
		t.Fatalf("expected start error, got %v", err)
	}
	if len(client.killCalls) != 0 {
		t.Fatalf("did not expect kill for execWithInput start error")
	}
}

func TestExecWithInputCreateError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > file"},
				createErr: errors.New("create failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	call := client.execQueue[0]
	if err := sbx.execWithInput(context.Background(), "cid", "cat > file", []byte("data")); !errors.Is(err, call.createErr) {
		t.Fatalf("expected create error, got %v", err)
	}
}

func TestExecWithInputAttachError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > file"},
				attachErr: errors.New("attach failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	call := client.execQueue[0]
	if err := sbx.execWithInput(context.Background(), "cid", "cat > file", []byte("data")); !errors.Is(err, call.attachErr) {
		t.Fatalf("expected attach error, got %v", err)
	}
}

func TestExecWithInputWriteError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > file"},
				writeErr:  errors.New("write failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	if err := sbx.execWithInput(context.Background(), "cid", "cat > file", []byte("data")); err == nil || err.Error() != "write failed" {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestExecWithInputInspectError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd:  []string{"/bin/sh", "-c", "cat > file"},
				inspectErr: errors.New("inspect failed"),
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	if err := sbx.execWithInput(context.Background(), "cid", "cat > file", []byte("data")); err == nil || err.Error() != "inspect failed" {
		t.Fatalf("expected inspect error, got %v", err)
	}
}

func TestExecWithInputExitNonZero(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > file"},
				inspect:   types.ContainerExecInspect{ExitCode: 3},
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{}}
	err := sbx.execWithInput(context.Background(), "cid", "cat > file", []byte("data"))
	if err == nil || !strings.Contains(err.Error(), "exit=3") {
		t.Fatalf("expected exit error, got %v", err)
	}
}

func TestSandboxRunSuccess(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "chmod 600 '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"python3", "main.py"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
				stdout:    "out\n",
				stderr:    "err\n",
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{WallTime: time.Second, MemoryB: 1024, NanoCPUs: 1}}

	var stdoutBuf, stderrBuf strings.Builder
	exit, timedOut, err := sbx.Run(
		context.Background(),
		"main.py",
		[]byte("print('hi')"),
		[][]string{{"python3", "main.py"}},
		func(p []byte) { stdoutBuf.Write(p) },
		func(p []byte) { stderrBuf.Write(p) },
	)
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}
	if exit != 0 || timedOut {
		t.Fatalf("unexpected exit=%d timedOut=%v", exit, timedOut)
	}
	if stdoutBuf.String() != "out\n" || stderrBuf.String() != "err\n" {
		t.Fatalf("unexpected stdout/stderr: %q %q", stdoutBuf.String(), stderrBuf.String())
	}
	if !client.removed {
		t.Fatalf("expected container removal")
	}
	if got := client.executed[1].stdin.String(); got != "print('hi')" {
		t.Fatalf("expected code to be written, got %q", got)
	}
	if !client.executed[1].conn.closeWrite {
		t.Fatalf("expected CloseWrite to be called")
	}
}

func TestSandboxRunCommandFailure(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "chmod 600 '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"python3", "main.py"},
				inspect:   types.ContainerExecInspect{ExitCode: 2},
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{WallTime: time.Second, MemoryB: 1024, NanoCPUs: 1}}
	exit, timedOut, err := sbx.Run(
		context.Background(),
		"main.py",
		[]byte("code"),
		[][]string{{"python3", "main.py"}},
		func([]byte) {},
		func([]byte) {},
	)
	if err != nil {
		t.Fatalf("expected nil error on non-zero exit, got %v", err)
	}
	if exit != 2 || timedOut {
		t.Fatalf("expected exit=2 timedOut=false, got exit=%d timedOut=%v", exit, timedOut)
	}
}

func TestSandboxRunCopyFailure(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 1},
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{WallTime: time.Second, MemoryB: 1024, NanoCPUs: 1}}
	_, _, err := sbx.Run(
		context.Background(),
		"main.py",
		[]byte("code"),
		[][]string{{"python3", "main.py"}},
		func([]byte) {},
		func([]byte) {},
	)
	if err == nil {
		t.Fatalf("expected error when copyFile fails")
	}
	if len(client.killCalls) != 1 || client.killCalls[0] != "SIGKILL" {
		t.Fatalf("expected container kill on failure")
	}
}

func TestSandboxRunContainerCreateError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createErr:  errors.New("create failed"),
		createResp: container.ContainerCreateCreatedBody{},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{WallTime: time.Second}}
	_, _, err := sbx.Run(context.Background(), "main.py", []byte("code"), [][]string{{"python3", "main.py"}}, func([]byte) {}, func([]byte) {})
	if !errors.Is(err, client.createErr) {
		t.Fatalf("expected create error, got %v", err)
	}
	if len(client.killCalls) != 0 {
		t.Fatalf("did not expect kill on create error")
	}
}

func TestSandboxRunContainerStartError(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		startErr:   errors.New("start failed"),
		execQueue:  []*fakeExecCall{},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{WallTime: time.Second}}
	_, _, err := sbx.Run(context.Background(), "main.py", []byte("code"), [][]string{{"python3", "main.py"}}, func([]byte) {}, func([]byte) {})
	if !errors.Is(err, client.startErr) {
		t.Fatalf("expected start error, got %v", err)
	}
	if !client.removed {
		t.Fatalf("expected container removal despite start error")
	}
}

func TestSandboxRunEnsureImageError(t *testing.T) {
	stub := &fakeDockerClient{
		t:               t,
		imageInspectErr: client.ErrorConnectionFailed("unix:///var/run/docker.sock"),
	}
	sbx := &Sandbox{cli: stub, image: "image", limits: Limits{WallTime: time.Second}}
	exit, timedOut, err := sbx.Run(context.Background(), "main.py", []byte("code"), [][]string{{"python3", "main.py"}}, func([]byte) {}, func([]byte) {})
	if !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
	if exit != -1 || timedOut {
		t.Fatalf("expected exit=-1 timedOut=false, got exit=%d timedOut=%v", exit, timedOut)
	}
}

func TestSandboxRunExecStartError(t *testing.T) {
	stub := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "chmod 600 '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"python3", "main.py"},
				createErr: errors.New("exec failed"),
			},
		},
	}
	sbx := &Sandbox{cli: stub, image: "image", limits: Limits{WallTime: time.Second}}
	call := stub.execQueue[3]
	_, _, err := sbx.Run(context.Background(), "main.py", []byte("code"), [][]string{{"python3", "main.py"}}, func([]byte) {}, func([]byte) {})
	if !errors.Is(err, call.createErr) {
		t.Fatalf("expected exec error, got %v", err)
	}
	if len(stub.killCalls) == 0 {
		t.Fatalf("expected container kill on execStart error")
	}
}

func TestSandboxRunInspectError(t *testing.T) {
	stub := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "chmod 600 '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd:  []string{"python3", "main.py"},
				inspectErr: errors.New("inspect failed"),
			},
		},
	}
	sbx := &Sandbox{cli: stub, image: "image", limits: Limits{WallTime: time.Second}}
	_, _, err := sbx.Run(context.Background(), "main.py", []byte("code"), [][]string{{"python3", "main.py"}}, func([]byte) {}, func([]byte) {})
	if err == nil || err.Error() != "inspect failed" {
		t.Fatalf("expected inspect error, got %v", err)
	}
	if len(stub.killCalls) == 0 {
		t.Fatalf("expected container kill on inspect error")
	}
}

func TestSandboxRunNoCommands(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "chmod 600 '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
		},
	}
	sbx := &Sandbox{cli: client, image: "image", limits: Limits{WallTime: time.Second, MemoryB: 1024, NanoCPUs: 1}}
	exit, timedOut, err := sbx.Run(
		context.Background(),
		"main.py",
		[]byte("code"),
		nil,
		func([]byte) {},
		func([]byte) {},
	)
	if err != nil || exit != 0 || timedOut {
		t.Fatalf("expected zero exit without commands, got exit=%d timedOut=%v err=%v", exit, timedOut, err)
	}
}

func TestExecuteUnsupportedLanguage(t *testing.T) {
	if _, err := Execute(context.Background(), Language("nope"), "code", Limits{}); err == nil {
		t.Fatalf("expected error for unsupported language")
	}
}

func TestExecuteSandboxUnavailable(t *testing.T) {
	orig := newDockerClient
	newDockerClient = func() (dockerClient, error) {
		return nil, client.ErrorConnectionFailed("unix:///var/run/docker.sock")
	}
	defer func() { newDockerClient = orig }()

	res, err := Execute(context.Background(), LangPython, "code", Limits{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.Error != "sandbox_unavailable" {
		t.Fatalf("expected sandbox_unavailable, got %q", res.Error)
	}
	if len(res.Events) != 2 || res.Events[0].Type != "error" || res.Events[1].Type != "exit" {
		t.Fatalf("unexpected events: %+v", res.Events)
	}
	if res.Exit.Code != -1 {
		t.Fatalf("expected exit code -1, got %d", res.Exit.Code)
	}
}

func TestExecuteSuccessAndErrorEvent(t *testing.T) {
	client := &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "cat > '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"/bin/sh", "-c", "chmod 600 '/workspace/main.py'"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
			},
			{
				expectCmd: []string{"python3", "main.py"},
				inspect:   types.ContainerExecInspect{ExitCode: 0},
				stdout:    "hello",
				stderr:    "warn",
			},
		},
	}

	orig := newDockerClient
	newDockerClient = func() (dockerClient, error) {
		return client, nil
	}
	defer func() { newDockerClient = orig }()

	res, err := Execute(context.Background(), LangPython, "print('hi')", Limits{})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("expected empty error, got %q", res.Error)
	}
	if res.Exit.Code != 0 || res.Exit.TimedOut {
		t.Fatalf("unexpected exit info: %+v", res.Exit)
	}
	if res.Stdout != "hello" || res.Stderr != "warn" {
		t.Fatalf("expected stdout/stderr from execution, got %q and %q", res.Stdout, res.Stderr)
	}

	// Now force runtime error via Run returning error
	client = &fakeDockerClient{
		t:          t,
		createResp: container.ContainerCreateCreatedBody{ID: "cid"},
		execQueue: []*fakeExecCall{
			{
				expectCmd: []string{"/bin/sh", "-c", "mkdir -p '/workspace'"},
				inspect:   types.ContainerExecInspect{ExitCode: 1},
			},
		},
	}
	newDockerClient = func() (dockerClient, error) {
		return client, nil
	}
	res, err = Execute(context.Background(), LangPython, "print()", Limits{})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if res.Error != "sandbox_error" {
		t.Fatalf("expected sandbox_error when run fails, got %q", res.Error)
	}
	foundErrorEvent := false
	for _, ev := range res.Events {
		if ev.Type == "error" {
			foundErrorEvent = true
			break
		}
	}
	if !foundErrorEvent {
		t.Fatalf("expected error event when sandbox run fails")
	}
}

func TestDefaultDockerClientFactory(t *testing.T) {
	orig := newDockerClient
	defer func() { newDockerClient = orig }()

	cli, err := orig()
	if err != nil {
		if !errors.Is(translateDockerErr(err), ErrDockerUnavailable) {
			t.Fatalf("unexpected error from default factory: %v", err)
		}
		return
	}
	if real, ok := cli.(*client.Client); ok {
		real.Close()
	}
}

func TestNewSandboxDefaults(t *testing.T) {
	orig := newDockerClient
	newDockerClient = func() (dockerClient, error) {
		return &fakeDockerClient{t: t}, nil
	}
	defer func() { newDockerClient = orig }()

	sbx, err := NewSandbox("image", Limits{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sbx.limits.WallTime <= 0 || sbx.limits.MemoryB == 0 || sbx.limits.NanoCPUs == 0 {
		t.Fatalf("expected defaults to be applied, got %+v", sbx.limits)
	}
}

func TestNewSandboxPropagatesErrors(t *testing.T) {
	orig := newDockerClient
	newDockerClient = func() (dockerClient, error) {
		return nil, client.ErrorConnectionFailed("unix:///var/run/docker.sock")
	}
	defer func() { newDockerClient = orig }()

	_, err := NewSandbox("image", Limits{})
	if !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("expected ErrDockerUnavailable, got %v", err)
	}
}

type fakeDockerClient struct {
	t               *testing.T
	imageInspectErr error
	imagePullErr    error
	imagePulled     bool

	createResp container.ContainerCreateCreatedBody
	createErr  error
	startErr   error
	removed    bool

	execQueue []*fakeExecCall
	executed  []*fakeExecCall
	execMap   map[string]*fakeExecCall

	killCalls []string
}

type fakeExecCall struct {
	expectCmd []string
	gotCmd    []string

	createErr  error
	attachErr  error
	startErr   error
	inspect    types.ContainerExecInspect
	inspectErr error

	stdout string
	stderr string

	stdin    bytes.Buffer
	conn     *fakeConn
	writeErr error
}

func (f *fakeDockerClient) ImageInspectWithRaw(context.Context, string) (types.ImageInspect, []byte, error) {
	return types.ImageInspect{}, nil, f.imageInspectErr
}

func (f *fakeDockerClient) ImagePull(context.Context, string, types.ImagePullOptions) (io.ReadCloser, error) {
	if f.imagePullErr != nil {
		return nil, f.imagePullErr
	}
	f.imagePulled = true
	return io.NopCloser(strings.NewReader("ok")), nil
}

func (f *fakeDockerClient) ContainerCreate(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, string) (container.ContainerCreateCreatedBody, error) {
	return f.createResp, f.createErr
}

func (f *fakeDockerClient) ContainerRemove(context.Context, string, types.ContainerRemoveOptions) error {
	f.removed = true
	return nil
}

func (f *fakeDockerClient) ContainerStart(context.Context, string, types.ContainerStartOptions) error {
	return f.startErr
}

func (f *fakeDockerClient) ContainerKill(ctx context.Context, containerID string, signal string) error {
	f.killCalls = append(f.killCalls, signal)
	return nil
}

func (f *fakeDockerClient) ensureExecMap() {
	if f.execMap == nil {
		f.execMap = make(map[string]*fakeExecCall)
	}
}

func (f *fakeDockerClient) fail(format string, args ...interface{}) {
	if f.t != nil {
		f.t.Fatalf(format, args...)
	}
	panic(fmt.Sprintf(format, args...))
}

func (f *fakeDockerClient) nextExec(config types.ExecConfig) (*fakeExecCall, string, error) {
	if len(f.execQueue) == 0 {
		f.fail("unexpected exec create for command %v", config.Cmd)
	}
	call := f.execQueue[0]
	f.execQueue = f.execQueue[1:]
	call.gotCmd = append([]string(nil), config.Cmd...)
	if len(call.expectCmd) > 0 && !reflect.DeepEqual(call.expectCmd, config.Cmd) {
		f.fail("expected cmd %v, got %v", call.expectCmd, config.Cmd)
	}
	f.executed = append(f.executed, call)
	id := fmt.Sprintf("exec-%d", len(f.executed))
	return call, id, call.createErr
}

func (f *fakeDockerClient) ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error) {
	call, id, err := f.nextExec(config)
	if err != nil {
		return types.IDResponse{}, err
	}
	f.ensureExecMap()
	f.execMap[id] = call
	return types.IDResponse{ID: id}, nil
}

func (f *fakeDockerClient) ContainerExecAttach(ctx context.Context, execID string, config types.ExecStartCheck) (types.HijackedResponse, error) {
	call := f.execMap[execID]
	if call == nil {
		f.fail("attach called for unknown exec id %s", execID)
	}
	if call.attachErr != nil {
		return types.HijackedResponse{}, call.attachErr
	}
	conn := &fakeConn{buf: &call.stdin, call: call}
	call.conn = conn
	data := muxStreams(call.stdout, call.stderr)
	return types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(bytes.NewReader(data)),
	}, nil
}

func (f *fakeDockerClient) ContainerExecStart(ctx context.Context, execID string, config types.ExecStartCheck) error {
	call := f.execMap[execID]
	if call == nil {
		f.fail("start called for unknown exec id %s", execID)
	}
	return call.startErr
}

func (f *fakeDockerClient) ContainerExecInspect(ctx context.Context, execID string) (types.ContainerExecInspect, error) {
	call := f.execMap[execID]
	if call == nil {
		f.fail("inspect called for unknown exec id %s", execID)
	}
	if call.inspectErr != nil {
		return types.ContainerExecInspect{}, call.inspectErr
	}
	return call.inspect, nil
}

type fakeConn struct {
	buf        *bytes.Buffer
	closed     bool
	closeWrite bool
	call       *fakeExecCall
}

func (c *fakeConn) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (c *fakeConn) Write(p []byte) (int, error) {
	if c.call != nil && c.call.writeErr != nil {
		return 0, c.call.writeErr
	}
	if c.buf != nil {
		return c.buf.Write(p)
	}
	return len(p), nil
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

type fakeAddr string

func (a fakeAddr) Network() string { return string(a) }
func (a fakeAddr) String() string  { return string(a) }

func (c *fakeConn) LocalAddr() net.Addr  { return fakeAddr("local") }
func (c *fakeConn) RemoteAddr() net.Addr { return fakeAddr("remote") }
func (c *fakeConn) SetDeadline(time.Time) error {
	return nil
}
func (c *fakeConn) SetReadDeadline(time.Time) error {
	return nil
}
func (c *fakeConn) SetWriteDeadline(time.Time) error {
	return nil
}
func (c *fakeConn) CloseWrite() error {
	c.closeWrite = true
	return nil
}

func muxStreams(stdout, stderr string) []byte {
	var buf bytes.Buffer
	if stdout != "" {
		buf.Write(singleStream(1, stdout))
	}
	if stderr != "" {
		buf.Write(singleStream(2, stderr))
	}
	return buf.Bytes()
}

func singleStream(stream byte, payload string) []byte {
	data := []byte(payload)
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
	return append(header, data...)
}

func TestWarmImagesPullsDefaults(t *testing.T) {
	orig := newDockerClient
	defer func() { newDockerClient = orig }()

	var clients []*fakeDockerClient
	newDockerClient = func() (dockerClient, error) {
		c := &fakeDockerClient{
			t:               t,
			imageInspectErr: errdefs.NotFound(errors.New("missing")),
		}
		clients = append(clients, c)
		return c, nil
	}

	if err := WarmImages(context.Background()); err != nil {
		t.Fatalf("warm images error: %v", err)
	}
	if len(clients) != 3 {
		t.Fatalf("expected three warmup clients, got %d", len(clients))
	}
	for i, c := range clients {
		if !c.imagePulled {
			t.Fatalf("expected client %d to pull image", i)
		}
	}
}

func TestWarmImagesPropagatesErrors(t *testing.T) {
	orig := newDockerClient
	defer func() { newDockerClient = orig }()

	newDockerClient = func() (dockerClient, error) {
		return nil, ErrDockerUnavailable
	}
	if err := WarmImages(context.Background()); !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

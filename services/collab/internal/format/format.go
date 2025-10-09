package format

import (
	"context"

	"collab/internal/exec"
	"collab/internal/models"
)

func Format(ctx context.Context, req models.FormatRequest) (string, error) {
	limits := exec.SandboxLimits{
		WallTime: 3_000_000_000, // ~3s
		MemoryB:  256 * 1024 * 1024,
		NanoCPUs: 500_000_000, // ~0.5 CPU
	}

	var image, fileName string
	var cmd []string

	switch req.Language {
	case models.LangPython:
		image, fileName, cmd = "peerprep/python:latest", "main.py", []string{"bash", "-lc", "black --quiet main.py && cat main.py"}
	case models.LangJava:
		image, fileName, cmd = "peerprep/java:latest", "Main.java", []string{"bash", "-lc", "google-java-format -a Main.java > /tmp/fmt.java && mv /tmp/fmt.java Main.java && cat Main.java"}
	case models.LangCPP:
		image, fileName, cmd = "peerprep/cpp:latest", "main.cpp", []string{"bash", "-lc", "clang-format -style=Google main.cpp > /tmp/f.cpp && mv /tmp/f.cpp main.cpp && cat main.cpp"}
	default:
		// Unsupported language: return input unchanged
		return req.Code, nil
	}

	sbx, err := exec.NewSandbox(image, limits)
	if err != nil {
		return "", err
	}

	var buf string
	_, _, _ = sbx.Run(ctx, fileName, []byte(req.Code), [][]string{cmd},
		func(p []byte) { buf += string(p) },
		func(p []byte) {},
	)
	return buf, nil
}

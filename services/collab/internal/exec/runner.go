package exec

import (
	"context"
	"errors"
	"strings"

	"collab/internal/models"
)

type Runner struct{}

func NewRunner() *Runner { return &Runner{} }

type RunOutput struct {
	Stdout   string
	Stderr   string
	Exit     int
	TimedOut bool
}

func (r *Runner) LangSpecPublic(lang models.Language) (spec models.LanguageSpec, image string, fileName string, cmds [][]string, err error) {
	return r.langSpec(lang)
}

func (r *Runner) RunOnce(ctx context.Context, lang models.Language, code string, limits SandboxLimits) (RunOutput, error) {
	spec, image, fileName, cmds, err := r.langSpec(lang)
	if err != nil {
		return RunOutput{}, err
	}

	sbx, err := NewSandbox(image, limits)
	if err != nil {
		return RunOutput{}, err
	}
	var out, errS strings.Builder

	exit, timedOut, runErr := sbx.Run(ctx, fileName, []byte(code), cmds,
		func(p []byte) { out.Write(p) },
		func(p []byte) { errS.Write(p) },
	)
	_ = spec
	if runErr != nil && !timedOut {
		return RunOutput{}, runErr
	}

	return RunOutput{
		Stdout:   out.String(),
		Stderr:   errS.String(),
		Exit:     exit,
		TimedOut: timedOut,
	}, nil
}

func (r *Runner) langSpec(lang models.Language) (models.LanguageSpec, string, string, [][]string, error) {
	switch lang {
	case models.LangPython:
		return models.LanguageSpec{
				Name:            lang,
				FileName:        "main.py",
				RunCmd:          []string{"python3", "main.py"},
				DefaultTabSize:  4,
				Formatter:       []string{"black"},
				ExampleTemplate: "print(\"Hello from Python!\")\n",
			},
			"python:3.11-slim",
			"main.py",
			[][]string{{"python3", "main.py"}},
			nil

	case models.LangJava:
		return models.LanguageSpec{
				Name:            lang,
				FileName:        "Main.java",
				CompileCmd:      []string{"javac", "Main.java"},
				ExecCmd:         []string{"/bin/sh", "-c", "java Main"},
				DefaultTabSize:  4,
				Formatter:       []string{"google-java-format"},
				ExampleTemplate: "public class Main {\n    public static void main(String[] args) {\n        System.out.println(\"Hello from Java!\");\n    }\n}\n",
			},
			"eclipse-temurin:17-jdk",
			"Main.java",
			[][]string{{"javac", "Main.java"}, {"/bin/sh", "-c", "java Main"}},
			nil

	case models.LangCPP:
		return models.LanguageSpec{
				Name:            lang,
				FileName:        "main.cpp",
				CompileCmd:      []string{"g++", "-O2", "-std=c++17", "main.cpp", "-o", "main"},
				ExecCmd:         []string{"./main"},
				DefaultTabSize:  2,
				Formatter:       []string{"clang-format"},
				ExampleTemplate: "#include <iostream>\n\nint main() {\n    std::cout << \"Hello from C++!\" << std::endl;\n    return 0;\n}\n",
			},
			"gcc:13",
			"main.cpp",
			[][]string{{"g++", "-O2", "-std=c++17", "main.cpp", "-o", "main"}, {"./main"}},
			nil
	default:
		return models.LanguageSpec{}, "", "", nil, errors.New("unsupported language")
	}
}

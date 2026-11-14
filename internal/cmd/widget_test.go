package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWidgetsPreserveBuffersAndCheckActualStatus(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			binary, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s unavailable", shell)
			}
			script, err := filepath.Abs(filepath.Join("..", "..", shell+"rc-snippet.sh"))
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				status, command, want string
				cursor                int
			}{{"0", "inserted", "left insertedright", 13}, {"0", "", "left right", 5}, {"1", "not inserted", "left right", 5}} {
				t.Run(tc.status+"/"+tc.command, func(t *testing.T) {
					program := `bind(){ :; }; bindkey(){ :; }; zle(){ :; }; cs(){ printf '%s' "$FAKE_COMMAND"; return "$FAKE_STATUS"; }; source "$SCRIPT"; `
					if shell == "bash" {
						program += `READLINE_LINE='left right'; READLINE_POINT=5; cs-insert-widget; test "$READLINE_LINE" = "$WANT" && test "$READLINE_POINT" = "$CURSOR"`
					} else {
						program += `LBUFFER='left '; RBUFFER='right'; cs-insert-widget; test "${LBUFFER}${RBUFFER}" = "$WANT" && test "$RBUFFER" = right && test "${#LBUFFER}" = "$CURSOR"`
					}
					command := exec.Command(binary, "-c", program)
					command.Env = []string{"PATH=/usr/bin:/bin", "SCRIPT=" + script, "FAKE_STATUS=" + tc.status, "FAKE_COMMAND=" + tc.command, "WANT=" + tc.want, "CURSOR=" + fmt.Sprint(tc.cursor), "HOME=" + t.TempDir()}
					if output, err := command.CombinedOutput(); err != nil {
						t.Fatalf("widget %v: %s", err, output)
					}
				})
			}
			t.Run("UTF8-character-offset-binding", func(t *testing.T) {
				// This tests the documented character-offset binding, not Readline's
				// version-dependent byte offsets (characterized separately in a real PTY).
				program := `s='é界'; test "${#s}" = 2 || exit 77; bind(){ :; }; bindkey(){ :; }; zle(){ :; }; cs(){ printf %s 'ø雪'; }; source "$SCRIPT"; `
				if shell == "bash" {
					program += `READLINE_LINE='é界 右'; READLINE_POINT=3; cs-insert-widget; test "$READLINE_LINE" = 'é界 ø雪右' && test "$READLINE_POINT" = 5`
				} else {
					program += `LBUFFER='é界 '; RBUFFER='右'; cs-insert-widget; test "$LBUFFER" = 'é界 ø雪' && test "$RBUFFER" = '右' && test "${#LBUFFER}" = 5`
				}
				command := exec.Command(binary, "-c", program)
				command.Env = []string{"PATH=/usr/bin:/bin", "SCRIPT=" + script, "HOME=" + t.TempDir(), "LC_ALL=C.UTF-8"}
				output, err := command.CombinedOutput()
				if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 77 {
					t.Skip("C.UTF-8 character counting unavailable")
				}
				if err != nil {
					t.Fatalf("Unicode binding: %v: %s", err, output)
				}
			})
			if shell == "bash" {
				t.Run("UTF8-byte-offset-caveat", func(t *testing.T) {
					program := `s='é界'; test "${#s}" = 2 || exit 77; bind(){ :; }; cs(){ printf %s 'ø雪'; }; source "$SCRIPT"; READLINE_LINE='é界 右'; READLINE_POINT=6; cs-insert-widget; test "$READLINE_LINE" = 'é界 右ø雪' && test "$READLINE_POINT" = 8`
					command := exec.Command(binary, "-c", program)
					command.Env = []string{"PATH=/usr/bin:/bin", "SCRIPT=" + script, "HOME=" + t.TempDir(), "LC_ALL=C.UTF-8"}
					output, err := command.CombinedOutput()
					if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 77 {
						t.Skip("C.UTF-8 character counting unavailable")
					}
					if err != nil {
						t.Fatalf("documented byte-offset caveat changed: %v: %s", err, output)
					}
					t.Log("Forced byte offset reproduces documented unsupported insertion after right text; no compatibility expansion")
				})
			}
			t.Run("UTF8-real-editor-offset-characterization", func(t *testing.T) {
				if runtime.GOOS == "windows" {
					t.Skip("Unix controlling PTY unavailable")
				}
				python, err := exec.LookPath("python3")
				if err != nil {
					t.Skip("python3 unavailable for real PTY characterization")
				}
				harness, err := filepath.Abs(filepath.Join("..", "..", "scripts", "verify_widgets.py"))
				if err != nil {
					t.Fatal(err)
				}
				command := exec.Command(python, harness, shell, script)
				output, err := command.CombinedOutput()
				if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 77 {
					t.Skipf("PTY capability unavailable: %s", output)
				}
				if err != nil {
					t.Fatalf("real editor characterization: %v: %s", err, output)
				}
				t.Log(string(output))
			})
		})
	}
}

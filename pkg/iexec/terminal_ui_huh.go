package iexec

import (
	"os"

	"charm.land/huh/v2"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// HuhTerminal is the interactive picker. It draws on /dev/tty so a piped
// stdin or stdout (`kubectl iexec api env | sort`) stays untouched.
type HuhTerminal struct{}

// NewHuhTerminal returns the huh-based TerminalUI.
func NewHuhTerminal() *HuhTerminal {
	return &HuhTerminal{}
}

func (h *HuhTerminal) SelectPod(pods []corev1.Pod, config Config) (corev1.Pod, error) {
	if len(pods) == 1 {
		return pods[0], nil
	}
	options := make([]huh.Option[int], len(pods))
	for i, p := range pods {
		options[i] = huh.NewOption(p.Namespace+"/"+p.Name, i)
	}
	i, err := pick("Select Pod", options, config)
	if err != nil {
		return corev1.Pod{}, errors.Wrap(err, "pod selection failed")
	}
	return pods[i], nil
}

func (h *HuhTerminal) SelectContainer(containers []corev1.Container, config Config) (corev1.Container, error) {
	if len(containers) == 1 {
		return containers[0], nil
	}
	options := make([]huh.Option[int], len(containers))
	for i, c := range containers {
		options[i] = huh.NewOption(c.Name, i)
	}
	i, err := pick("Select Container", options, config)
	if err != nil {
		return corev1.Container{}, errors.Wrap(err, "container selection failed")
	}
	return containers[i], nil
}

// pick runs a single select on /dev/tty and returns the chosen option value.
func pick(title string, options []huh.Option[int], config Config) (int, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open /dev/tty")
	}
	defer tty.Close() //nolint:errcheck

	var choice int
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[int]().Title(title).Options(options...).Value(&choice),
	)).WithInput(tty).WithOutput(tty).WithTheme(theme(config))

	if err := form.Run(); err != nil {
		return 0, err
	}
	return choice, nil
}

func theme(config Config) huh.Theme {
	if config.Naked {
		return huh.ThemeFunc(huh.ThemeBase)
	}
	return huh.ThemeFunc(huh.ThemeCharm)
}

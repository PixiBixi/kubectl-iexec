# kubectl-iexec

[![CI](https://github.com/PixiBixi/kubectl-iexec/actions/workflows/ci.yml/badge.svg)](https://github.com/PixiBixi/kubectl-iexec/actions/workflows/ci.yml)
[![Lint](https://github.com/PixiBixi/kubectl-iexec/actions/workflows/lint.yml/badge.svg)](https://github.com/PixiBixi/kubectl-iexec/actions/workflows/lint.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Interactive `kubectl exec`: filter pods by name, pick a pod and a container from a list, and get a shell (or run a command) in it.

This is a maintained fork of [gabeduke/kubectl-iexec](https://github.com/gabeduke/kubectl-iexec), which has been inactive since 2023. All credit for the original design and code goes to gabeduke. The fork keeps the MIT license and the original copyright.

## What changed compared to upstream

- Go 1.27, client-go 0.37, current dependencies
- WebSocket exec with SPDY fallback (like modern `kubectl`), raw terminal mode and live resize
- Clean output when stdin or stdout is piped (`kubectl iexec api printenv | sort > .env`)
- `--first` to skip the prompts
- Pickers built on [huh](https://github.com/charmbracelet/huh): press `/` to filter on `namespace/name`
- Pods that are not `Running` or are terminating are skipped (`--all-phases` to keep them)
- Default shell is `bash` with a fallback to `sh` for alpine and busybox images
- `--debug` for images without a shell (distroless): attaches an ephemeral debug container instead of an exec
- Releases signed with cosign, with SBOMs and provenance, published to a Krew index

## Install

With [Krew](https://krew.sigs.k8s.io/):

```bash
kubectl krew index add pixibixi https://github.com/PixiBixi/krew-index
kubectl krew install pixibixi/iexec
```

With Go:

```bash
go install github.com/PixiBixi/kubectl-iexec/v2@latest
```

The binary must be named `kubectl-iexec` and be on your `PATH`. Prebuilt archives for Linux and macOS (amd64, arm64) are on the [releases page](https://github.com/PixiBixi/kubectl-iexec/releases).

### Shell completion

With kubectl 1.26 or later and kubectl completion enabled in your shell, `kubectl iexec <TAB>` completes pod names, workloads (`deploy/<TAB>`), `-n`, `--context` and `-c`. kubectl looks for a `kubectl_complete-iexec` executable on your `PATH`; this creates it as a symlink next to the plugin (once, it survives krew upgrades):

```bash
kubectl iexec --install-completion
```

## Usage

```text
kubectl iexec [pod filter] [remote command(s)] [flags]
```

The first argument filters pods by name, or selects the pods of a workload (`deploy/api`, `sts/db`, `ds/agent`, `rs/x`, `job/x`). The remaining arguments are the command to run (default: `bash` when the image has it, `sh` otherwise). Put `--` before a command that takes flags, otherwise iexec reads them (`-c` is `--container`). The exit code of the remote command is returned.

```bash
# pick from all pods in the namespace, then open a shell
kubectl iexec

# pick from pods matching "busybox"
kubectl iexec busybox

# run a command
kubectl iexec busybox cat /etc/hosts

# pick among the pods of a workload (deploy/, sts/, ds/, rs/, job/)
kubectl iexec deploy/api

# pick a pod, then a container matching "second_container"
kubectl iexec multi-container-pod -c second_container

# no prompt: first matching pod and container
kubectl iexec busybox --first cat /etc/hosts

# no shell in the image (distroless): attach an ephemeral busybox sharing its processes
kubectl iexec api --debug

# a command with its own flags goes after --
kubectl iexec busybox -- sh -c 'echo $HOSTNAME'
```

| Flag | Description |
| --- | --- |
| `-c, --container` | Container name filter |
| `-l, --selector` | Label selector to filter pods (`key1=value1,key2=value2`) |
| `--first` | Skip the prompts and use the first matching pod and container |
| `--debug` | Attach an ephemeral debug container targeting the selected container instead of an exec. Needs `patch` on `pods/ephemeralcontainers` |
| `--debug-image` | Image of the `--debug` container (default `busybox:1.37`) |
| `--all-phases` | Include pods that are not `Running` or are terminating (skipped by default) |
| `-A, --all-namespaces` | Search pods across all namespaces |
| `-n, --namespace` | Namespace to search |
| `-v, --vim-mode` | Deprecated, no effect: `j`/`k` always navigate |
| `-x, --naked` | Decolorize output |
| `--install-completion` | Link `kubectl_complete-iexec` next to the plugin for shell completion, then exit |
| `--log-level` | `trace`, `debug`, `info`, `warn`, `error`, `fatal` or `panic` |

The usual kubectl connection flags (`--context`, `--kubeconfig`, `--server`, ...) are also accepted.

## Development

```bash
make build   # build ./kubectl-iexec
make test    # go test -race ./...
make lint    # golangci-lint
```

See [AGENTS.md](AGENTS.md) for the full workflow and [SECURITY.md](SECURITY.md) to report a vulnerability.

## License

MIT, see [LICENSE](LICENSE). Original copyright notice kept.

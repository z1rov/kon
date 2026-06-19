# kron-core

CLI wrapper for [kron-image](https://github.com/z1rov/kron-image).

## Install

```bash
git clone https://github.com/z1rov/kron-core
cd kron-core
make install
```

Or build manually:
```bash
go build -o kron ./cmd/kron
sudo mv kron /usr/local/bin/
```

## Commands

### Container

| Command | Description |
|---|---|
| `kron start` | Start container (host network, `~/kron-forge` mounted as `/forge`) |
| `kron start --ad` | Start with `NET_RAW` + `NET_ADMIN` for AD tools (Responder, mitm6, PCredz) |
| `kron stop` | Stop the running container |
| `kron status` | Show container state, image info, forge path |
| `kron logs` | Show last 50 lines of container logs |
| `kron logs -f` | Follow container logs |
| `kron exec <cmd>` | Run a command inside the container |

### Image

| Command | Description |
|---|---|
| `kron install` | Pull the latest kron image |
| `kron update` | Compare local vs remote version and pull if newer |
| `kron version` | Show local and remote versions |

## How versioning works

`kron update` and `kron version` fetch the `banner.txt` from the kron-image repo and parse the `Version:` field. It compares that to the version inside the local image's own `banner.txt`. If they differ, it pulls.

## Notes

- Forge directory (`~/kron-forge`) is created automatically on first `kron start`
- If the container is already running, `kron start` attaches to it instead of creating a new one
- `--network host` is always used so the container shares the host's interfaces
- `/etc/hosts` is always bind-mounted so host entries are visible inside

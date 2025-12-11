# Container Runtime on macOS - How It Works

## Our Setup: Docker CLI + Podman

We use a **FOSS (Free and Open Source Software)** approach:
- **Docker CLI** (`brew install docker`) - Just the CLI for k3d compatibility
- **Podman** (`brew install podman`) - The actual container runtime/daemon
- **k3d** - Uses Docker CLI, which connects to Podman via `DOCKER_HOST`

## Architecture

**Podman on macOS:**
1. Podman runs in a **Linux VM** (using QEMU)
2. The Podman daemon runs **inside that VM**
3. The Docker CLI on macOS connects to Podman via a **socket**
4. k3d uses Docker CLI, which talks to Podman
5. The socket path is set via `DOCKER_HOST` environment variable

## How It Works

When you run `./scripts/setup-macos-env.sh`:
1. Installs Docker CLI (homebrew)
2. Installs Podman (homebrew)
3. Initializes Podman machine (`podman machine init`)
4. Starts Podman machine (`podman machine start`)
5. Configures `DOCKER_HOST` to point to Podman socket

When you run `./scripts/start-k3d.sh`:
1. Auto-configures `DOCKER_HOST` if not set
2. k3d uses Docker CLI
3. Docker CLI connects to Podman via `DOCKER_HOST`
4. k3d creates k3s cluster in Podman containers
5. kubectl connects to k3s API (not Podman directly)

## The Solution

**If Podman machine is not running:**
```bash
podman machine start
```

**If DOCKER_HOST is not set:**
```bash
# Get Podman socket
PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')

# Set DOCKER_HOST (for current session)
export DOCKER_HOST="unix://${PODMAN_SOCKET}"

# Or add to ~/.zshrc for persistence:
echo 'export DOCKER_HOST="unix://${PODMAN_SOCKET}"' >> ~/.zshrc
```

**Verify everything works:**
```bash
./scripts/check-docker.sh  # Shows Docker CLI, Podman, and connection status
./scripts/start-k3d.sh      # Creates cluster using Podman
```

## Where Is Everything?

- **Podman VM**: Runs in background (QEMU VM)
- **Podman containers**: Inside the VM
- **k3d cluster**: k3s running in Podman containers inside the VM
- **kubectl**: Connects to k3s API (not Podman directly)

## The Key Point

- **kubectl** connects to Kubernetes API (works if cluster exists)
- **k3d** needs container runtime access (via Docker CLI → Podman)
- **Podman machine** must be running for k3d to work
- **DOCKER_HOST** must be set to Podman socket

Run `./scripts/check-docker.sh` to see what's actually happening with your container runtime.


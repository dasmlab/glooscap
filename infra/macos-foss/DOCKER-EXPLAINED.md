# Docker on macOS - How It Works

## The Confusion

You're right to be confused! Here's what's happening:

## Docker Architecture on macOS

**Docker Desktop on macOS:**
1. Docker Desktop runs in a **Linux VM** (using HyperKit or similar)
2. The Docker daemon runs **inside that VM**
3. The Docker CLI on macOS connects to the daemon via a **socket**
4. The socket is typically at `/var/run/docker.sock` (but Docker Desktop uses a different path)

## Why It Worked Before

When you first created the cluster:
- Docker Desktop was running
- k3d could access the Docker daemon
- Cluster was created in Docker containers inside the VM
- kubeconfig was created pointing to the cluster

## Why It Doesn't Work Now

After removing the cluster:
- kubeconfig is gone (kubectl can't connect)
- Script tries to create a new cluster
- k3d needs Docker daemon access
- Docker daemon might not be accessible from current shell context

## The Solution

**IMPORTANT: Docker CLI vs Docker Desktop**

- `brew install docker` → **CLI only** (no daemon, won't work with k3d)
- `brew install --cask docker` → **Docker Desktop** (includes daemon, works with k3d)

**If you installed via `brew install docker` (CLI only):**
```bash
# Uninstall CLI-only version
brew uninstall docker

# Install Docker Desktop (includes daemon)
brew install --cask docker

# Start Docker Desktop
open -a Docker

# Wait for it to start, then verify:
./scripts/check-docker.sh
./scripts/start-k3d.sh
```

**If Docker Desktop is already installed:**
```bash
# Just start it
open -a Docker
# Wait for it to start, then:
./scripts/check-docker.sh  # Verify Docker is accessible
./scripts/start-k3d.sh     # Create cluster
```

**Option 2: Check Docker Context**
```bash
docker context ls
docker context use default  # Or whatever context has Docker
```

**Option 3: Check Docker Socket**
```bash
./scripts/check-docker.sh  # Shows where Docker socket is
```

## Where Is Everything?

- **Docker Desktop VM**: Runs in background (HyperKit VM)
- **Docker containers**: Inside the VM
- **k3d cluster**: k3s running in Docker containers inside the VM
- **kubectl**: Connects to k3s API (not Docker directly)

## The Key Point

- **kubectl** connects to Kubernetes API (works if cluster exists)
- **k3d** needs Docker daemon access (to create/manage clusters)
- **Docker Desktop** must be running for k3d to work

Run `./scripts/check-docker.sh` to see what's actually happening with Docker on your system.


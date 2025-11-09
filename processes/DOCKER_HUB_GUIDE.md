# Docker Hub Push Guide

This guide provides step-by-step instructions for pushing your CloudAI process images to Docker Hub.

## Prerequisites

1. **Docker Hub Account**: Create a free account at https://hub.docker.com
2. **Docker Installed**: Verify with `docker --version`
3. **Built Images**: Images must be built locally first

## Step-by-Step Guide

### Step 1: Create Docker Hub Account

1. Visit https://hub.docker.com/signup
2. Create your account (free plan is sufficient)
3. Verify your email address
4. Remember your Docker Hub username (you'll need it)

### Step 2: Login to Docker Hub

Open your terminal and run:

```bash
docker login
```

Enter your Docker Hub username and password when prompted.

**Verify you're logged in:**
```bash
docker info | grep Username
```

You should see your username displayed.

### Step 3: Set Your Docker Hub Username

Before building, set your username as an environment variable:

```bash
export DOCKER_USERNAME="your_actual_username"
```

Replace `your_actual_username` with your Docker Hub username.

**Make it permanent (optional):**

Add to your `~/.bashrc` or `~/.zshrc`:
```bash
echo 'export DOCKER_USERNAME="your_actual_username"' >> ~/.bashrc
source ~/.bashrc
```

### Step 4: Build All Images

Navigate to the processes directory:

```bash
cd /home/moin/Projects/CloudAI/processes
```

Make the build script executable:

```bash
chmod +x build-local.sh
```

Build all images:

```bash
./build-local.sh
```

This will build 20 images and will take 10-30 minutes depending on your system.

**Verify images were built:**
```bash
docker images | grep cloudai
```

You should see all 20 images listed.

### Step 5: Push Images to Docker Hub

**Option A: Use the push script (Recommended)**

```bash
chmod +x push-images.sh
./push-images.sh
```

**Option B: Push manually**

```bash
# Push IO-intensive images
for i in {1..7}; do
    docker push $DOCKER_USERNAME/cloudai-io-intensive:$i
done

# Push CPU-intensive images
for i in {1..7}; do
    docker push $DOCKER_USERNAME/cloudai-cpu-intensive:$i
done

# Push GPU-intensive images
for i in {1..6}; do
    docker push $DOCKER_USERNAME/cloudai-gpu-intensive:$i
done
```

**Option C: Use the all-in-one script**

```bash
chmod +x build-and-push.sh
./build-and-push.sh
```

This script will:
1. Build all images
2. Ask if you want to push
3. Push all images to Docker Hub

### Step 6: Verify Upload

1. Visit Docker Hub: https://hub.docker.com
2. Click "Repositories"
3. You should see your three repositories:
   - `cloudai-io-intensive`
   - `cloudai-cpu-intensive`
   - `cloudai-gpu-intensive`

### Step 7: Get Image Links

Your images are now available at these URLs:

**Web Interface:**
- https://hub.docker.com/r/`<username>`/cloudai-io-intensive
- https://hub.docker.com/r/`<username>`/cloudai-cpu-intensive
- https://hub.docker.com/r/`<username>`/cloudai-gpu-intensive

**Pull Commands:**
```bash
docker pull <username>/cloudai-io-intensive:1
docker pull <username>/cloudai-cpu-intensive:1
docker pull <username>/cloudai-gpu-intensive:1
```

**Direct Image URLs (for APIs):**
```
docker.io/<username>/cloudai-io-intensive:1
docker.io/<username>/cloudai-cpu-intensive:1
docker.io/<username>/cloudai-gpu-intensive:1
```

Replace `<username>` with your actual Docker Hub username.

## Getting Specific Image Links

### Format

All your images follow this format:
```
docker.io/<username>/<repository>:<tag>
```

### Complete List

Replace `USERNAME` with your Docker Hub username:

#### IO-Intensive Processes
```
docker.io/USERNAME/cloudai-io-intensive:1
docker.io/USERNAME/cloudai-io-intensive:2
docker.io/USERNAME/cloudai-io-intensive:3
docker.io/USERNAME/cloudai-io-intensive:4
docker.io/USERNAME/cloudai-io-intensive:5
docker.io/USERNAME/cloudai-io-intensive:6
docker.io/USERNAME/cloudai-io-intensive:7
docker.io/USERNAME/cloudai-io-intensive:latest
```

#### CPU-Intensive Processes
```
docker.io/USERNAME/cloudai-cpu-intensive:1
docker.io/USERNAME/cloudai-cpu-intensive:2
docker.io/USERNAME/cloudai-cpu-intensive:3
docker.io/USERNAME/cloudai-cpu-intensive:4
docker.io/USERNAME/cloudai-cpu-intensive:5
docker.io/USERNAME/cloudai-cpu-intensive:6
docker.io/USERNAME/cloudai-cpu-intensive:7
docker.io/USERNAME/cloudai-cpu-intensive:latest
```

#### GPU-Intensive Processes
```
docker.io/USERNAME/cloudai-gpu-intensive:1
docker.io/USERNAME/cloudai-gpu-intensive:2
docker.io/USERNAME/cloudai-gpu-intensive:3
docker.io/USERNAME/cloudai-gpu-intensive:4
docker.io/USERNAME/cloudai-gpu-intensive:5
docker.io/USERNAME/cloudai-gpu-intensive:6
docker.io/USERNAME/cloudai-gpu-intensive:latest
```

### Using Links in CloudAI

When assigning tasks in your CloudAI system, use these image URLs:

```go
// Example
task := &pb.Task{
    TaskId: "task-001",
    DockerImage: "johndoe/cloudai-cpu-intensive:3",  // Replace 'johndoe' with your username
    Command: "python cpu_process_3.py",
    ReqCpu: 4,
    ReqMemory: 8,
}
```

## Sharing Your Images

### Make Public (Default)
Your images are public by default - anyone can pull them.

### Make Private
1. Go to https://hub.docker.com
2. Navigate to your repository
3. Click "Settings"
4. Change "Visibility" to "Private"

**Note:** Free Docker Hub accounts get 1 private repository.

### Share with Others

To share your images, give others:
- Repository URL: `https://hub.docker.com/r/<username>/cloudai-io-intensive`
- Pull command: `docker pull <username>/cloudai-io-intensive:1`

## Updating Images

### Rebuild and Push Updates

```bash
# Rebuild all
./build-local.sh

# Push updates
./push-images.sh
```

### Tag Management

```bash
# Add new tags
docker tag $DOCKER_USERNAME/cloudai-io-intensive:1 $DOCKER_USERNAME/cloudai-io-intensive:v1.0
docker push $DOCKER_USERNAME/cloudai-io-intensive:v1.0

# Update latest tag
docker tag $DOCKER_USERNAME/cloudai-io-intensive:1 $DOCKER_USERNAME/cloudai-io-intensive:latest
docker push $DOCKER_USERNAME/cloudai-io-intensive:latest
```

## Troubleshooting

### Authentication Failed
```bash
# Logout and login again
docker logout
docker login
```

### Image Not Found After Push
- Wait 1-2 minutes for Docker Hub to index
- Refresh the repository page
- Verify push completed successfully

### Push Denied
- Verify you're logged in: `docker info | grep Username`
- Check repository name matches your username
- Ensure you have write permissions

### Slow Push Speed
- First push uploads all layers (slow)
- Subsequent pushes only upload changes (fast)
- Large GPU images (6-8 GB) take longer

### Rate Limiting
Docker Hub has pull rate limits (100 pulls/6 hours for free accounts).

To check rate limit status:
```bash
TOKEN=$(curl -s "https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull" | jq -r .token)
curl -s --head -H "Authorization: Bearer $TOKEN" https://registry-1.docker.io/v2/ratelimitpreview/test/manifests/latest | grep RateLimit
```

## Quick Reference

### Essential Commands

```bash
# Login
docker login

# Build all
cd /home/moin/Projects/CloudAI/processes
./build-local.sh

# Push all
./push-images.sh

# Verify
docker images | grep cloudai

# Test pull
docker pull $DOCKER_USERNAME/cloudai-io-intensive:1

# Run test
docker run --rm -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-io-intensive:1
```

### Image URL Template

```
docker.io/<your-username>/cloudai-<type>:<number>
```

Examples:
- `docker.io/johndoe/cloudai-io-intensive:1`
- `docker.io/johndoe/cloudai-cpu-intensive:5`
- `docker.io/johndoe/cloudai-gpu-intensive:3`

## Next Steps

1. ✅ Verify all images are on Docker Hub
2. ✅ Test pulling an image from Docker Hub
3. ✅ Update your CloudAI master configuration with image URLs
4. ✅ Test task assignment with these images
5. ✅ Monitor resource usage during execution

## Additional Resources

- Docker Hub: https://hub.docker.com
- Docker Documentation: https://docs.docker.com
- Docker Hub Limits: https://docs.docker.com/docker-hub/download-rate-limit/

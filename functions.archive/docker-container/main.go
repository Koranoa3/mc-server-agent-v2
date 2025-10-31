package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func main() {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		panic(err)
	}

	if len(containers) == 0 {
		fmt.Println("no containers found")
		return
	}

	// Print list
	for _, c := range containers {
		fmt.Printf("Container ID: %s, Image: %s, Status: %s\n", c.ID[:12], c.Image, c.Status)
	}

	// Choose 3rd container to exec into (safe command)
	target := containers[2]
	fmt.Printf("\nExecuting harmless command in container %s (%s)\n", target.ID[:12], target.Image)

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", "echo hello-from-agent && ls -la / | head -n 5"},
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := cli.ContainerExecCreate(ctx, target.ID, execConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create exec: %v\n", err)
		return
	}

	attachResp, err := cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to attach to exec: %v\n", err)
		return
	}
	defer attachResp.Close()

	// Stream the output to stdout (so it appears in container logs)
	if _, err := io.Copy(os.Stdout, attachResp.Reader); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read exec output: %v\n", err)
		return
	}

	// Optionally inspect exec to get exit code
	insp, err := cli.ContainerExecInspect(ctx, execResp.ID)
	if err == nil {
		fmt.Printf("\nExec finished with exit code: %d\n", insp.ExitCode)
	}
}

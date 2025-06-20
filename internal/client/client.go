package client

import (
	"context"
	"time"

	pb "distributed-task-scheduler/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TaskClient struct {
	client pb.TaskSchedulerClient
}

func NewTaskClient(address string) (*TaskClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewTaskSchedulerClient(conn)
	return &TaskClient{client: client}, nil
}

func (c *TaskClient) SubmitTask(name, command string, scheduleTime time.Time) (string, error) {
	task := &pb.Task{
		Id:           time.Now().Format("20060102150405"),
		Name:         name,
		Command:      command,
		ScheduleTime: scheduleTime.Unix(),
	}

	resp, err := c.client.SubmitTask(context.Background(), task)
	if err != nil {
		return "", err
	}

	return resp.TaskId, nil
}

func (c *TaskClient) GetTaskStatus(taskId string) (*pb.TaskStatus, error) {
	status, err := c.client.GetTaskStatus(context.Background(), &pb.TaskId{Id: taskId})
	if err != nil {
		return nil, err
	}

	return status, nil
}

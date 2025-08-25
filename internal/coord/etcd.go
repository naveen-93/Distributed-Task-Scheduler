package coord

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// LeaderCallbacks defines actions to perform when gaining or losing leadership.
type LeaderCallbacks struct {
	// OnStartedLeading is called when leadership is acquired. It is given a context that
	// will be canceled when leadership is lost or Run is stopped.
	OnStartedLeading func(ctx context.Context)
}

// EtcdLeader coordinates leader election using etcd.
type EtcdLeader struct {
	endpoints    []string
	namespace    string
	electionName string
	leaseTTL     time.Duration
}

func NewEtcdLeader(endpoints []string, namespace, electionName string, leaseTTL time.Duration) *EtcdLeader {
	return &EtcdLeader{
		endpoints:    endpoints,
		namespace:    namespace,
		electionName: electionName,
		leaseTTL:     leaseTTL,
	}
}

// Run performs leader election and invokes callbacks while leader.
func (e *EtcdLeader) Run(ctx context.Context, cb LeaderCallbacks) error {
	if len(e.endpoints) == 0 {
		return fmt.Errorf("no etcd endpoints provided")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   e.endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		sess, err := concurrency.NewSession(cli, concurrency.WithTTL(int(e.leaseTTL.Seconds())))
		if err != nil {
			// brief backoff before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}

		election := concurrency.NewElection(sess, path.Join(e.namespace, e.electionName))

		// campaign to become leader
		ident := leaderIdentity()
		if err := election.Campaign(ctx, ident); err != nil {
			sess.Close()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// retry campaign on transient errors
			continue
		}

		// became leader; run callback with a context tied to session
		leaderCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		if cb.OnStartedLeading != nil {
			go func() {
				cb.OnStartedLeading(leaderCtx)
				close(done)
			}()
		}

		// wait for session end or context cancel
		select {
		case <-ctx.Done():
			cancel()
			_ = election.Resign(context.Background())
			_ = sess.Close()
			<-done
			return ctx.Err()
		case <-sess.Done():
			cancel()
			_ = election.Resign(context.Background())
			_ = sess.Close()
			<-done
			// loop to re-enter election
			continue
		}
	}
}

func leaderIdentity() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

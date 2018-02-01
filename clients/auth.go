package clients

import (
	"fmt"

	"context"

	"io"

	"git.containerum.net/ch/grpc-proto-files/auth"
	"git.containerum.net/ch/grpc-proto-files/common"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// AuthSvc is an interface to auth service
type AuthSvc interface {
	UpdateUserAccess(ctx context.Context, userID string) error

	// for connections closing
	io.Closer
}

type authSvcGRPC struct {
	client auth.AuthClient
	addr   string
	log    *logrus.Entry
	conn   *grpc.ClientConn
}

// NewAuthSvcGRPC creates grpc client to auth service. It does nothing but logs actions.
func NewAuthSvcGRPC(addr string) (as AuthSvc, err error) {
	ret := authSvcGRPC{
		log:  logrus.WithField("component", "auth_client"),
		addr: addr,
	}

	ret.log.Debugf("grpc connect to %s", addr)
	ret.conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithUnaryInterceptor(grpc_logrus.UnaryClientInterceptor(ret.log)))
	if err != nil {
		return
	}
	ret.client = auth.NewAuthClient(ret.conn)

	return ret, nil
}

func (as authSvcGRPC) UpdateUserAccess(ctx context.Context, userID string) error {
	as.log.WithField("user_id", userID).Infoln("update user access")
	_, err := as.client.UpdateAccess(ctx, &auth.UpdateAccessRequest{
		UserId: &common.UUID{Value: userID},
	})
	return err
}

func (as authSvcGRPC) String() string {
	return fmt.Sprintf("auth grpc client: addr=%v", as.addr)
}

func (as authSvcGRPC) Close() error {
	return as.conn.Close()
}

type authSvcDummy struct {
	log *logrus.Entry
}

// NewDummyAuthSvc creates dummy auth client
func NewDummyAuthSvc() AuthSvc {
	return authSvcDummy{
		log: logrus.WithField("component", "auth_stub"),
	}
}

func (as authSvcDummy) UpdateUserAccess(ctx context.Context, userID string) error {
	as.log.WithField("user_id", userID).Infoln("update user access")
	return nil
}

func (authSvcDummy) String() string {
	return "ch-auth client dummy"
}

func (authSvcDummy) Close() error {
	return nil
}
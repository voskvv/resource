package clients

import (
	"context"
	"fmt"
	"net/url"

	kubtypesInternal "git.containerum.net/ch/kube-api/pkg/model"
	"git.containerum.net/ch/kube-client/pkg/cherry"
	"git.containerum.net/ch/kube-client/pkg/cherry/adaptors/cherrylog"
	"git.containerum.net/ch/kube-client/pkg/cherry/resource-service"
	kubtypes "git.containerum.net/ch/kube-client/pkg/model"
	"git.containerum.net/ch/utils"
	"github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"gopkg.in/resty.v1"
)

// Kube is an interface to kube-api service
type Kube interface {
	CreateNamespace(ctx context.Context, ns kubtypesInternal.NamespaceWithOwner) error
	SetNamespaceQuota(ctx context.Context, ns kubtypesInternal.NamespaceWithOwner) error
	DeleteNamespace(ctx context.Context, label string) error

	CreateDeployment(ctx context.Context, nsLabel string, deploy kubtypesInternal.DeploymentWithOwner) error
	DeleteDeployment(ctx context.Context, nsLabel, deplLabel string) error
	ReplaceDeployment(ctx context.Context, nsLabel, deplLabel string, deploy kubtypesInternal.DeploymentWithOwner) error
	SetDeploymentReplicas(ctx context.Context, nsLabel, deplLabel string, replicas int) error
	SetContainerImage(ctx context.Context, nsLabel, deplLabel string, container kubtypes.Container) error

	CreateIngress(ctx context.Context, nsLabel string, ingress kubtypesInternal.IngressWithOwner) error
	DeleteIngress(ctx context.Context, nsLabel, ingressName string) error

	CreateSecret(ctx context.Context, nsLabel string, secret kubtypesInternal.SecretWithOwner) error
	DeleteSecret(ctx context.Context, nsLabel, secretName string) error
}

type kube struct {
	client *resty.Client
	log    *cherrylog.LogrusAdapter
}

// NewKubeHTTP creates http client to kube-api service.
func NewKubeHTTP(u *url.URL) Kube {
	log := logrus.WithField("component", "kube_client")
	client := resty.New().
		SetHostURL(u.String()).
		SetLogger(log.WriterLevel(logrus.DebugLevel)).
		SetDebug(true).
		SetError(cherry.Err{})
	client.JSONMarshal = jsoniter.Marshal
	client.JSONUnmarshal = jsoniter.Unmarshal
	return kube{
		client: client,
		log:    cherrylog.NewLogrusAdapter(log),
	}
}

func (kub kube) CreateNamespace(ctx context.Context, ns kubtypesInternal.NamespaceWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"cpu":    ns.Resources.Hard.CPU,
		"memory": ns.Resources.Hard.Memory,
		"name":   ns.Label,
		"access": ns.Access,
	}).Debug("create namespace")

	resp, err := kub.client.R().
		SetBody(ns).
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		Post("/namespaces")
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) DeleteNamespace(ctx context.Context, label string) error {
	kub.log.WithField("label", label).Debug("delete namespace")

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		Delete("/namespaces/" + url.PathEscape(label))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) SetNamespaceQuota(ctx context.Context, ns kubtypesInternal.NamespaceWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"cpu":    ns.Resources.Hard.CPU,
		"memory": ns.Resources.Hard.Memory,
		"label":  ns.Label,
	}).Debug("set namespace quota")

	resp, err := kub.client.R().
		SetBody(ns).
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		Put("/namespaces/" + url.PathEscape(ns.Label))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) CreateDeployment(ctx context.Context, nsLabel string, deploy kubtypesInternal.DeploymentWithOwner) error {
	kub.log.WithField("ns_label", nsLabel).Debug("create deployment %+v", deploy)

	resp, err := kub.client.R().
		SetBody(deploy).
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		Post(fmt.Sprintf("/namespaces/%s/deployments", nsLabel))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) DeleteDeployment(ctx context.Context, nsLabel, deplLabel string) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"deploy_label": deplLabel,
	}).Debug("delete deployment")

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		Delete(fmt.Sprintf("/namespaces/%s/deployments/%s", nsLabel, deplLabel))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) ReplaceDeployment(ctx context.Context, nsLabel, deplLabel string, deploy kubtypesInternal.DeploymentWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"deploy_label": deplLabel,
	}).Debug("replace deployment %+v", deploy)

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		SetBody(deploy).
		Put(fmt.Sprintf("/namespaces/%s/deployments/%s", nsLabel, deplLabel))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) SetDeploymentReplicas(ctx context.Context, nsLabel, deplLabel string, replicas int) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"deploy_label": deplLabel,
		"replicas":     replicas,
	}).Debug("change replicas")

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		SetBody(kubtypes.UpdateReplicas{Replicas: replicas}).
		Put(fmt.Sprintf("/namespaces/%s/deployments/%s/replicas", nsLabel, deplLabel))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) SetContainerImage(ctx context.Context, nsLabel, deplLabel string, container kubtypes.Container) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"deploy_label": deplLabel,
		"container":    container.Name,
		"image":        container.Image,
	}).Debug("set container image")

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		SetBody(container).
		Put(fmt.Sprintf("/namespaces/%s/deployments/%s/image", nsLabel, deplLabel))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) CreateIngress(ctx context.Context, nsLabel string, ingress kubtypesInternal.IngressWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label": nsLabel,
	}).Debugf("create ingress %+v", ingress)

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		SetBody(ingress).
		Post(fmt.Sprintf("/namespaces/%s/ingresses", nsLabel))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) DeleteIngress(ctx context.Context, nsLabel, ingressName string) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"ingress_name": ingressName,
	}).Debug("delete ingress")

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		Delete(fmt.Sprintf("/namespaces/%s/ingresses", nsLabel))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) CreateSecret(ctx context.Context, nsLabel string, secret kubtypesInternal.SecretWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label": nsLabel,
	}).Debugf("create secret %+v", secret)

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		SetBody(secret).
		Post(fmt.Sprintf("/namespaces/%s/secrets", nsLabel))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}
	return nil
}

func (kub kube) DeleteSecret(ctx context.Context, nsLabel, secretName string) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":    nsLabel,
		"secret_name": secretName,
	}).Debug("delete secret")

	resp, err := kub.client.R().
		SetContext(ctx).
		SetHeaders(utils.RequestHeadersMap(ctx)).
		Delete(fmt.Sprintf("/namespaces/%s/secrets/%s", nsLabel, secretName))
	if err != nil {
		return rserrors.ErrInternal().Log(err, kub.log)
	}
	if resp.Error() != nil {
		return resp.Error().(*cherry.Err)
	}

	return nil
}

func (kub kube) String() string {
	return fmt.Sprintf("kube api http client: url=%v", kub.client.HostURL)
}

// Dummy implementation

type kubeDummy struct {
	log *logrus.Entry
}

// NewDummyKube creates a dummy client to kube-api service. It does nothing but logs actions.
func NewDummyKube() Kube {
	return kubeDummy{log: logrus.WithField("component", "kube_stub")}
}

func (kub kubeDummy) CreateNamespace(_ context.Context, ns kubtypesInternal.NamespaceWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"cpu":    ns.Resources.Hard.CPU,
		"memory": ns.Resources.Hard.Memory,
		"name":   ns.Label,
		"access": ns.Access,
	}).Debug("create namespace")
	return nil
}

func (kub kubeDummy) DeleteNamespace(_ context.Context, label string) error {
	kub.log.WithField("label", label).Debug("delete namespace")
	return nil
}

func (kub kubeDummy) SetNamespaceQuota(_ context.Context, ns kubtypesInternal.NamespaceWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"cpu":    ns.Resources.Hard.CPU,
		"memory": ns.Resources.Hard.Memory,
		"label":  ns.Label,
	}).Debug("set namespace quota")

	return nil
}

func (kub kubeDummy) CreateDeployment(_ context.Context, nsLabel string, deploy kubtypesInternal.DeploymentWithOwner) error {
	kub.log.WithField("ns_label", nsLabel).Debug("create deployment %+v", deploy)

	return nil
}

func (kub kubeDummy) DeleteDeployment(_ context.Context, nsLabel, deplLabel string) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"deploy_label": deplLabel,
	}).Debug("delete deployment")

	return nil
}

func (kub kubeDummy) ReplaceDeployment(_ context.Context, nsLabel, deplLabel string, deploy kubtypesInternal.DeploymentWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"deploy_label": deplLabel,
	}).Debug("replace deployment %+v", deploy)

	return nil
}

func (kub kubeDummy) SetDeploymentReplicas(ctx context.Context, nsLabel, deplLabel string, replicas int) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"deploy_label": deplLabel,
		"replicas":     replicas,
	}).Debug("change replicas")

	return nil
}

func (kub kubeDummy) SetContainerImage(ctx context.Context, nsLabel, deplLabel string, container kubtypes.Container) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"deploy_label": deplLabel,
		"container":    container.Name,
		"image":        container.Image,
	}).Debug("set container image")

	return nil
}

func (kub kubeDummy) CreateIngress(ctx context.Context, nsLabel string, ingress kubtypesInternal.IngressWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label": nsLabel,
	}).Debugf("create ingress %+v", ingress)

	return nil
}

func (kub kubeDummy) DeleteIngress(ctx context.Context, nsLabel, ingressName string) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":     nsLabel,
		"ingress_name": ingressName,
	}).Debug("delete ingress")

	return nil
}

func (kub kubeDummy) CreateSecret(ctx context.Context, nsLabel string, secret kubtypesInternal.SecretWithOwner) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label": nsLabel,
	}).Debugf("create secret %+v", secret)

	return nil
}

func (kub kubeDummy) DeleteSecret(ctx context.Context, nsLabel, secretName string) error {
	kub.log.WithFields(logrus.Fields{
		"ns_label":    nsLabel,
		"secret_name": secretName,
	}).Debug("delete secret")

	return nil
}

func (kubeDummy) String() string {
	return "kube api dummy"
}
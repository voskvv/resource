package models

import (
	"context"

	"git.containerum.net/ch/auth/proto"
	"git.containerum.net/ch/json-types/kube-api"
	rstypes "git.containerum.net/ch/json-types/resource-service"
	kubtypes "git.containerum.net/ch/kube-client/pkg/model"
	"git.containerum.net/ch/utils"
	"github.com/jmoiron/sqlx"
)

type RelationalDB interface {
	sqlx.ExtContext
	utils.SQLXPreparer

	Transactional(ctx context.Context, f func(ctx context.Context, tx RelationalDB) error) error
}

/* Namespace DB */

type NamespaceDB interface {
	CreateNamespace(ctx context.Context, userID, label string, namespace *rstypes.Namespace) error
	GetUserNamespaces(ctx context.Context, userID string, filters *NamespaceFilterParams) ([]rstypes.NamespaceWithVolumes, error)
	GetAllNamespaces(ctx context.Context, page, perPage int, filters *NamespaceFilterParams) ([]rstypes.NamespaceWithVolumes, error)
	GetUserNamespaceByLabel(ctx context.Context, userID, label string) (rstypes.NamespaceWithPermission, error)
	GetUserNamespaceWithVolumesByLabel(ctx context.Context, userID string, label string) (rstypes.NamespaceWithVolumes, error)
	GetNamespaceWithUserPermissions(ctx context.Context, userID, label string) (rstypes.NamespaceWithUserPermissions, error)
	DeleteUserNamespaceByLabel(ctx context.Context, userID, label string) (rstypes.Namespace, error)
	DeleteAllUserNamespaces(ctx context.Context, userID string) error
	RenameNamespace(ctx context.Context, userID, oldLabel, newLabel string) error
	ResizeNamespace(ctx context.Context, namespace *rstypes.Namespace) error
	GetNamespaceID(ctx context.Context, userID, nsLabel string) (string, error)
	GetNamespaceUsage(ctx context.Context, ns rstypes.Namespace) (NamespaceUsage, error)
}

type NamespaceDBConstructor func(RelationalDB) NamespaceDB

/* Volume DB */

type VolumeDB interface {
	CreateVolume(ctx context.Context, userID, label string, volume *rstypes.Volume) error
	GetUserVolumes(ctx context.Context, userID string, filters *VolumeFilterParams) ([]rstypes.VolumeWithPermission, error)
	GetAllVolumes(ctx context.Context, page, perPage int, filters *VolumeFilterParams) ([]rstypes.VolumeWithPermission, error)
	GetUserVolumeByLabel(ctx context.Context, userID, label string) (rstypes.VolumeWithPermission, error)
	GetVolumeWithUserPermissions(ctx context.Context, userID, label string) (rstypes.VolumeWithUserPermissions, error)
	GetVolumesLinkedWithUserNamespace(ctx context.Context, userID, label string) ([]rstypes.VolumeWithPermission, error)
	DeleteUserVolumeByLabel(ctx context.Context, userID, label string) (rstypes.Volume, error)
	DeleteAllUserVolumes(ctx context.Context, userID string, nonPersistentOnly bool) ([]rstypes.Volume, error)
	RenameVolume(ctx context.Context, userID, oldLabel, newLabel string) error
	ResizeVolume(ctx context.Context, volume *rstypes.Volume) error
	SetVolumeActiveByID(ctx context.Context, id string, active bool) error
	SetUserVolumeActive(ctx context.Context, userID, label string, active bool) error
	GetVolumeID(ctx context.Context, userID, label string) (string, error)
}

type VolumeDBConstructor func(RelationalDB) VolumeDB

/* Access DB */

type AccessDB interface {
	GetUserResourceAccesses(ctx context.Context, userID string) (*authProto.ResourcesAccess, error)
	GetUserResourceAccess(ctx context.Context, userID string, resourceKind rstypes.Kind, resourceName string) (rstypes.PermissionStatus, error)
	SetAllResourcesAccess(ctx context.Context, userID string, access rstypes.PermissionStatus) error
	SetResourceAccess(ctx context.Context, permRec *rstypes.PermissionRecord) error
	DeleteResourceAccess(ctx context.Context, resource rstypes.Resource, userID string) error
}

type AccessDBConstructor func(RelationalDB) AccessDB

/* Deploy DB */

type DeployDB interface {
	CreateDeployment(ctx context.Context, userID, nsLabel string, deployment kubtypes.Deployment) (bool, error)
	GetDeployments(ctx context.Context, userID, nsLabel string) ([]kubtypes.Deployment, error)
	GetDeploymentByLabel(ctx context.Context, userID, nsLabel, deplName string) (kubtypes.Deployment, error)
	DeleteDeployment(ctx context.Context, userID, nsLabel, deplName string) (bool, error)
	ReplaceDeployment(ctx context.Context, userID, nsLabel string, deploy kubtypes.Deployment) error
	SetDeploymentReplicas(ctx context.Context, userID, nsLabel, deplName string, replicas int) error
	SetContainerImage(ctx context.Context, userID, nsLabel, deplName string, req kubtypes.UpdateImage) error
	GetDeployID(ctx context.Context, nsID, deplName string) (string, error)
}

type DeployDBConstructor func(RelationalDB) DeployDB

/* Domain DB */

type DomainDB interface {
	AddDomain(ctx context.Context, req rstypes.AddDomainRequest) error
	GetAllDomains(ctx context.Context, params rstypes.GetAllDomainsQueryParams) ([]rstypes.Domain, error)
	GetDomain(ctx context.Context, domain string) (rstypes.Domain, error)
	DeleteDomain(ctx context.Context, domain string) error
	ChooseRandomDomain(ctx context.Context) (rstypes.Domain, error)
	ChooseDomainFreePort(ctx context.Context, domain string, protocol kubtypes.Protocol) (int, error)
}

type DomainDBConstructor func(RelationalDB) DomainDB

/* Ingress DB */

type IngressDB interface {
	CreateIngress(ctx context.Context, userID, nsLabel string, req rstypes.CreateIngressRequest) error
	GetUserIngresses(ctx context.Context, userID, nsLabel string, params rstypes.GetIngressesQueryParams) ([]rstypes.Ingress, error)
	GetAllIngresses(ctx context.Context, params rstypes.GetIngressesQueryParams) ([]rstypes.Ingress, error)
	GetIngress(ctx context.Context, userID, nsLabel, serviceName string) (rstypes.IngressEntry, error)
	DeleteIngress(ctx context.Context, userID, nsLabel, domain string) (rstypes.IngressType, error)
}

type IngressDBConstructor func(RelationalDB) IngressDB

/* Storage DB */

type StorageDB interface {
	CreateStorage(ctx context.Context, req rstypes.CreateStorageRequest) error
	GetStorages(ctx context.Context) ([]rstypes.Storage, error)
	UpdateStorage(ctx context.Context, name string, req rstypes.UpdateStorageRequest) error
	DeleteStorage(ctx context.Context, name string) error
	ChooseAvailableStorage(ctx context.Context, minFree int) (rstypes.Storage, error)
}

type StorageDBConstructor func(RelationalDB) StorageDB

/* Gluster Endpoints DB */

type GlusterEndpointsDB interface {
	CreateGlusterEndpoints(ctx context.Context, userID, nsLabel string) ([]kube_api.Endpoint, error)
	ConfirmGlusterEndpoints(ctx context.Context, userID, nsLabel string) error
}

type GlusterEndpointsDBConstructor func(RelationalDB) GlusterEndpointsDB

/* Service DB */

type ServiceDB interface {
	CreateService(ctx context.Context, userID, nsLabel string, serviceType rstypes.ServiceType, req kubtypes.Service) error
	GetServices(ctx context.Context, userID, nsLabel string) ([]kubtypes.Service, error)
	GetService(ctx context.Context, userID, nsLabel, serviceName string) (kubtypes.Service, rstypes.ServiceType, error)
	UpdateService(ctx context.Context, userID, nsLabel string, newServiceType rstypes.ServiceType, req kubtypes.Service) error
	DeleteService(ctx context.Context, userID, nsLabel, serviceName string) error
}

type ServiceDBConstructor func(RelationalDB) ServiceDB

/* Resource count DB */

type ResourceCountDB interface {
	GetResourcesCount(ctx context.Context, userID string) (rstypes.GetResourcesCountResponse, error)
}

type ResourceCountDBConstructor func(db RelationalDB) ResourceCountDB

type NamespaceUsage struct {
	CPU         int `db:"cpu"`
	RAM         int `db:"ram"`
	ExtServices int `db:"extservices"`
	IntServices int `db:"intservices"`
}

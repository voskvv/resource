package impl

import (
	"context"

	"git.containerum.net/ch/resource-service/pkg/clients"
	"git.containerum.net/ch/resource-service/pkg/db"
	"git.containerum.net/ch/resource-service/pkg/models/deployment"
	"git.containerum.net/ch/resource-service/pkg/rsErrors"
	"git.containerum.net/ch/resource-service/pkg/server"
	"github.com/blang/semver"
	"github.com/containerum/cherry/adaptors/cherrylog"
	"github.com/containerum/kube-client/pkg/diff"
	kubtypes "github.com/containerum/kube-client/pkg/model"
	"github.com/containerum/utils/httputil"
	"github.com/sirupsen/logrus"
)

type DeployActionsImpl struct {
	kube        clients.Kube
	permissions clients.Permissions
	mongo       *db.MongoStorage
	log         *cherrylog.LogrusAdapter
}

func NewDeployActionsImpl(mongo *db.MongoStorage, permissions *clients.Permissions, kube *clients.Kube) *DeployActionsImpl {
	return &DeployActionsImpl{
		kube:        *kube,
		mongo:       mongo,
		permissions: *permissions,
		log:         cherrylog.NewLogrusAdapter(logrus.WithField("component", "deploy_actions")),
	}
}

func (da *DeployActionsImpl) GetDeploymentsList(ctx context.Context, nsID string) (deployment.DeploymentList, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":   userID,
		"namespace": nsID,
	}).Info("get deployments")

	return da.mongo.GetDeploymentList(nsID)
}

func (da *DeployActionsImpl) GetDeployment(ctx context.Context, nsID, deplName string) (*deployment.DeploymentResource, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deplName,
	}).Info("get deployment by label")

	ret, err := da.mongo.GetDeployment(nsID, deplName)

	return &ret, err
}

func (da *DeployActionsImpl) GetDeploymentVersion(ctx context.Context, nsID, deplName, version string) (*deployment.DeploymentResource, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deplName,
	}).Info("get deployment by label")

	deplVersion, err := semver.Parse(version)
	if err != nil {
		return nil, err
	}
	ret, err := da.mongo.GetDeploymentVersion(nsID, deplName, deplVersion)

	return &ret, err
}

func (da *DeployActionsImpl) GetDeploymentVersionsList(ctx context.Context, nsID, deployName string) (deployment.DeploymentList, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":    userID,
		"namespace":  nsID,
		"deployment": deployName,
	}).Info("get deployments")

	return da.mongo.GetDeploymentVersionsList(nsID, deployName)
}

func (da *DeployActionsImpl) CreateDeployment(ctx context.Context, nsID string, deploy kubtypes.Deployment) (*deployment.DeploymentResource, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id": userID,
		"ns_id":   nsID,
	}).Info("create deployment")

	nsLimits, err := da.permissions.GetNamespaceLimits(ctx, nsID)
	if err != nil {
		return nil, err
	}

	nsUsage, err := da.mongo.GetNamespaceResourcesLimits(nsID)
	if err != nil {
		return nil, err
	}

	if err := server.CheckDeploymentCreateQuotas(nsLimits, nsUsage, deploy); err != nil {
		return nil, err
	}

	server.CalculateDeployResources(&deploy)

	deploy.Version = semver.MustParse("1.0.0")
	deploy.Active = true

	createdDeploy, err := da.mongo.CreateDeployment(deployment.DeploymentFromKube(nsID, userID, deploy))
	if err != nil {
		return nil, err
	}

	if err := da.kube.CreateDeployment(ctx, nsID, deploy); err != nil {
		da.log.Debug("Kube-API error! Deleting deployment from DB.")
		if err := da.mongo.DeleteDeployment(nsID, deploy.Name); err != nil {
			return nil, err
		}
		return nil, err
	}

	return &createdDeploy, nil
}

func (da *DeployActionsImpl) UpdateDeployment(ctx context.Context, nsID string, deploy kubtypes.Deployment) (*deployment.DeploymentResource, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deploy.Name,
	}).Infof("replacing deployment with %#v", deploy)

	server.CalculateDeployResources(&deploy)

	nsLimits, err := da.permissions.GetNamespaceLimits(ctx, nsID)
	if err != nil {
		return nil, err
	}

	nsUsage, err := da.mongo.GetNamespaceResourcesLimits(nsID)
	if err != nil {
		return nil, err
	}

	oldDeploy, err := da.mongo.GetDeployment(nsID, deploy.Name)
	if err != nil {
		return nil, err
	}

	if err := server.CheckDeploymentReplaceQuotas(nsLimits, nsUsage, oldDeploy.Deployment, deploy); err != nil {
		return nil, err
	}

	oldLatestDeploy, err := da.mongo.GetDeploymentLatestVersion(nsID, deploy.Name)
	if err != nil {
		return nil, err
	}

	oldversion := oldLatestDeploy.Deployment.Version

	deploy.Version = diff.NewVersion(oldLatestDeploy.Deployment, deploy)
	deploy.Active = true

	newversion := deploy.Version

	var updatedDeploy deployment.DeploymentResource
	if !newversion.Equals(oldversion) {
		if err := da.mongo.DeactivateDeployment(nsID, deploy.Name); err != nil {
			return nil, err
		}

		updatedDeploy, err = da.mongo.CreateDeployment(deployment.DeploymentFromKube(nsID, userID, deploy))
		if err != nil {
			return nil, err
		}

		if err := da.kube.UpdateDeployment(ctx, nsID, deploy); err != nil {
			da.log.Debug("Kube-API error! Reverting changes.")
			if err := da.mongo.DeleteDeploymentVersion(nsID, deploy.Name, newversion); err != nil {
				return nil, err
			}
			if err := da.mongo.ActivateDeployment(nsID, deploy.Name, oldDeploy.Version); err != nil {
				return nil, err
			}
			return nil, err
		}
	} else {
		if err := da.mongo.UpdateActiveDeployment(deployment.DeploymentFromKube(nsID, userID, deploy)); err != nil {
			return nil, err
		}
		updatedDeploy, err = da.mongo.GetDeployment(nsID, deploy.Name)
		if err != nil {
			return nil, err
		}

		if err := da.kube.UpdateDeployment(ctx, nsID, deploy); err != nil {
			da.log.Debug("Kube-API error! Reverting changes.")
			if err := da.mongo.UpdateActiveDeployment(oldDeploy); err != nil {
				return nil, err
			}
			return nil, err
		}
	}

	return &updatedDeploy, nil
}

func (da *DeployActionsImpl) SetDeploymentReplicas(ctx context.Context, nsID, deplName string, req kubtypes.UpdateReplicas) (*deployment.DeploymentResource, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deplName,
	}).Infof("set deployment replicas %#v", req)

	nsLimits, err := da.permissions.GetNamespaceLimits(ctx, nsID)
	if err != nil {
		return nil, err
	}

	nsUsage, err := da.mongo.GetNamespaceResourcesLimits(nsID)
	if err != nil {
		return nil, err
	}

	oldDeploy, err := da.mongo.GetDeployment(nsID, deplName)
	if err != nil {
		return nil, err
	}

	newDeploy := oldDeploy
	newDeploy.Replicas = req.Replicas
	newDeploy.Active = true
	if err := server.CheckDeploymentReplicasChangeQuotas(nsLimits, nsUsage, oldDeploy.Deployment, req.Replicas); err != nil {
		return nil, err
	}

	server.CalculateDeployResources(&newDeploy.Deployment)

	if err := da.mongo.UpdateActiveDeployment(newDeploy); err != nil {
		return nil, err
	}

	if err := da.kube.SetDeploymentReplicas(ctx, nsID, newDeploy.Name, req.Replicas); err != nil {
		da.log.Debug("Kube-API error! Reverting changes.")
		if err := da.mongo.UpdateActiveDeployment(oldDeploy); err != nil {
			return nil, err
		}
		return nil, err
	}

	updatedDeploy, err := da.mongo.GetDeployment(nsID, deplName)
	if err != nil {
		return nil, err
	}

	return &updatedDeploy, nil
}

func (da *DeployActionsImpl) RenameDeploymentVersion(ctx context.Context, nsID, deplName, oldversion, newversion string) (*deployment.DeploymentResource, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deplName,
		"old_version": oldversion,
		"new_version": newversion,
	}).Info("rename deployment version")

	oldDeplVersion, err := semver.Parse(oldversion)
	if err != nil {
		return nil, err
	}

	newDeplVersion, err := semver.Parse(newversion)
	if err != nil {
		return nil, err
	}

	if err := da.mongo.UpdateDeploymentVersion(nsID, deplName, oldDeplVersion, newDeplVersion); err != nil {
		return nil, err
	}

	updatedDeploy, err := da.mongo.GetDeploymentVersion(nsID, deplName, newDeplVersion)
	if err != nil {
		return nil, err
	}

	return &updatedDeploy, nil
}

func (da *DeployActionsImpl) SetDeploymentContainerImage(ctx context.Context, nsID, deplName string, req kubtypes.UpdateImage) (*deployment.DeploymentResource, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deplName,
	}).Infof("set container image %#v", req)

	oldDeploy, err := da.mongo.GetDeployment(nsID, deplName)
	if err != nil {
		return nil, err
	}

	newDeploy := oldDeploy

	updated := false
	for i, c := range newDeploy.Containers {
		if c.Name == req.Container {
			newDeploy.Containers[i].Image = req.Image
			updated = true
			break
		}
	}
	if !updated {
		return nil, rserrors.ErrNoContainer()
	}

	oldLatestDeploy, err := da.mongo.GetDeploymentLatestVersion(nsID, deplName)
	if err != nil {
		return nil, err
	}

	newDeploy.Version = diff.NewVersion(oldLatestDeploy.Deployment, newDeploy.Deployment)

	if err := da.mongo.DeactivateDeployment(nsID, newDeploy.Name); err != nil {
		return nil, err
	}

	updatedDeploy, err := da.mongo.CreateDeployment(newDeploy)
	if err != nil {
		return nil, err
	}

	if err := da.kube.UpdateDeployment(ctx, nsID, newDeploy.Deployment); err != nil {
		da.log.Debug("Kube-API error! Reverting changes.")
		if err := da.mongo.DeactivateDeployment(nsID, newDeploy.Name); err != nil {
			return nil, err
		}
		if err := da.mongo.ActivateDeployment(nsID, newDeploy.Name, oldDeploy.Version); err != nil {
			return nil, err
		}
		return nil, err
	}

	return &updatedDeploy, nil
}

func (da *DeployActionsImpl) ChangeActiveDeployment(ctx context.Context, nsID, deplName, version string) (*deployment.DeploymentResource, error) {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deplName,
	}).Infof("change active version %v", version)

	oldDeploy, err := da.mongo.GetDeployment(nsID, deplName)
	if err != nil {
		return nil, err
	}

	deplVersion, err := semver.Parse(version)
	if err != nil {
		return nil, err
	}

	newDeploy, err := da.mongo.GetDeploymentVersion(nsID, deplName, deplVersion)
	if err != nil {
		return nil, err
	}
	newDeploy.Active = true

	if err := da.mongo.DeactivateDeployment(nsID, newDeploy.Name); err != nil {
		return nil, err
	}

	if err := da.kube.UpdateDeployment(ctx, nsID, newDeploy.Deployment); err != nil {
		da.log.Debug("Kube-API error! Reverting changes.")
		if err := da.mongo.DeactivateDeployment(nsID, newDeploy.Name); err != nil {
			return nil, err
		}
		if err := da.mongo.ActivateDeployment(nsID, oldDeploy.Name, oldDeploy.Version); err != nil {
			return nil, err
		}
		return nil, err
	}

	if err := da.mongo.ActivateDeployment(nsID, newDeploy.Name, newDeploy.Version); err != nil {
		return nil, err
	}

	return &newDeploy, nil
}

func (da *DeployActionsImpl) DeleteDeployment(ctx context.Context, nsID, deplName string) error {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deplName,
	}).Info("delete deployment")

	if err := da.mongo.DeleteDeployment(nsID, deplName); err != nil {
		return err
	}

	if err := da.kube.DeleteDeployment(ctx, nsID, deplName); err != nil {
		da.log.Debug("Kube-API error! Reverting changes.")
		if err := da.mongo.RestoreDeployment(nsID, deplName); err != nil {
			return err
		}
		return err
	}

	return nil
}

func (da *DeployActionsImpl) DeleteDeploymentVersion(ctx context.Context, nsID, deplName, version string) error {
	userID := httputil.MustGetUserID(ctx)
	da.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"ns_id":       nsID,
		"deploy_name": deplName,
	}).Info("get deployment by label")

	deplVersion, err := semver.Parse(version)
	if err != nil {
		return err
	}

	activeDeploy, err := da.mongo.GetDeployment(nsID, deplName)
	if err == nil {
		if activeDeploy.Version.Equals(deplVersion) {
			return rserrors.ErrUnableDeleteActiveDeploymentVersion()
		}
	}

	return da.mongo.DeleteDeploymentVersion(nsID, deplName, deplVersion)
}

func (da *DeployActionsImpl) DeleteAllDeployments(ctx context.Context, nsID string) error {
	da.log.WithFields(logrus.Fields{
		"ns_id": nsID,
	}).Info("delete all deployments")

	if err := da.mongo.DeleteAllDeploymentsInNamespace(nsID); err != nil {
		return err
	}
	return nil
}

func (da *DeployActionsImpl) DiffDeployments(ctx context.Context, nsID, deplName, version1, version2 string) (*string, error) {
	da.log.WithFields(logrus.Fields{
		"ns_id":      nsID,
		"deployment": deplName,
		"version1":   version1,
		"version2":   version2,
	}).Info("diff deployment versions")

	v1, err := semver.Parse(version1)
	if err != nil {
		return nil, err
	}

	v2, err := semver.Parse(version2)
	if err != nil {
		return nil, err
	}

	depl1, err := da.mongo.GetDeploymentVersion(nsID, deplName, v1)
	if err != nil {
		return nil, err
	}

	depl2, err := da.mongo.GetDeploymentVersion(nsID, deplName, v2)
	if err != nil {
		return nil, err
	}

	deplDiff := diff.Diff(depl1.Deployment, depl2.Deployment)
	return &deplDiff, nil
}

func (da *DeployActionsImpl) DiffDeploymentsPrevious(ctx context.Context, nsID, deplName, version string) (*string, error) {
	da.log.WithFields(logrus.Fields{
		"ns_id":      nsID,
		"deployment": deplName,
		"version":    version,
	}).Info("diff deployment versions")

	v1, err := semver.Parse(version)
	if err != nil {
		return nil, err
	}

	deplList, err := da.mongo.GetDeploymentVersionsList(nsID, deplName)
	if err != nil {
		return nil, err
	}

	if len(deplList) == 0 {
		return nil, rserrors.ErrResourceNotExists()
	}

	if len(deplList) < 2 {
		return nil, rserrors.ErrOnlyOneDeploymentVersion()
	}

	var oldFound bool
	var prevFound bool
	var v2 semver.Version
	for _, d := range deplList {
		if oldFound {
			v2 = d.Version
			prevFound = true
			break
		}
		if d.Version.Equals(v1) {
			oldFound = true
		}
	}

	if !prevFound {
		return nil, rserrors.ErrResourceNotExists().AddDetails("no previous version found")
	}

	depl1, err := da.mongo.GetDeploymentVersion(nsID, deplName, v1)
	if err != nil {
		return nil, err
	}

	depl2, err := da.mongo.GetDeploymentVersion(nsID, deplName, v2)
	if err != nil {
		return nil, err
	}

	deplDiff := diff.Diff(depl1.Deployment, depl2.Deployment)
	return &deplDiff, nil
}

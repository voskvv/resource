package db

import (
	"git.containerum.net/ch/resource-service/pkg/models/ingress"
	"git.containerum.net/ch/resource-service/pkg/rsErrors"
	"github.com/containerum/kube-client/pkg/model"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/google/uuid"
)

func (mongo *MongoStorage) CreateIngress(ingress ingress.IngressResource) (ingress.IngressResource, error) {
	mongo.logger.Debugf("creating ingress")
	var collection = mongo.db.C(CollectionIngress)
	if ingress.ID == "" {
		ingress.ID = uuid.New().String()
	}
	if err := collection.Insert(ingress); err != nil {
		mongo.logger.WithError(err).Errorf("unable to create ingress")
		if mgo.IsDup(err) {
			return ingress, rserrors.ErrResourceAlreadyExists().AddDetailsErr(err)
		}
		return ingress, PipErr{err}.ToMongerr().Extract()
	}
	return ingress, nil
}

func (mongo *MongoStorage) GetIngress(namespaceID, name string) (ingress.IngressResource, error) {
	mongo.logger.Debugf("getting ingress")
	var collection = mongo.db.C(CollectionIngress)
	var ingr ingress.IngressResource
	if err := collection.Find(ingress.OneSelectQuery(namespaceID, name)).One(&ingr); err != nil {
		mongo.logger.WithError(err).Errorf("unable to get ingress")
		if err == mgo.ErrNotFound {
			return ingr, rserrors.ErrResourceNotExists().AddDetails(name)
		}
		return ingr, PipErr{err}.ToMongerr().Extract()
	}
	return ingr, nil
}

func (mongo *MongoStorage) GetIngressByService(namespaceID, serviceName string) (ingress.IngressResource, error) {
	mongo.logger.Debugf("getting ingress by service")
	var collection = mongo.db.C(CollectionIngress)
	var ingr ingress.IngressResource
	if err := collection.Find(bson.M{
		"namespaceid":                    namespaceID,
		"deleted":                        false,
		"ingress.rules.path.servicename": serviceName,
	}).One(&ingr); err != nil {
		mongo.logger.WithError(err).Errorf("unable to get ingress")
		if err == mgo.ErrNotFound {
			return ingr, rserrors.ErrResourceNotExists().AddDetails(serviceName)
		}
		return ingr, PipErr{err}.ToMongerr().Extract()
	}
	return ingr, nil
}

func (mongo *MongoStorage) GetIngressList(namespaceID string) (ingress.IngressList, error) {
	mongo.logger.Debugf("getting ingress")
	var collection = mongo.db.C(CollectionIngress)
	var list ingress.IngressList
	if err := collection.Find(ingress.ListSelectQuery(namespaceID)).All(&list); err != nil {
		mongo.logger.WithError(err).Errorf("unable to get ingress list")
		return list, PipErr{err}.ToMongerr().NotFoundToNil().Extract()
	}
	return list, nil
}

func (mongo *MongoStorage) UpdateIngress(upd ingress.IngressResource) (ingress.IngressResource, error) {
	mongo.logger.Debugf("updating ingress")
	var collection = mongo.db.C(CollectionIngress)
	if err := collection.Update(upd.OneSelectQuery(), upd.UpdateQuery()); err != nil {
		mongo.logger.WithError(err).Errorf("unable to update ingress")
		return upd, PipErr{err}.ToMongerr().Extract()
	}
	return upd, nil
}

func (mongo *MongoStorage) DeleteIngress(namespaceID, name string) error {
	mongo.logger.Debugf("deleting ingress")
	var collection = mongo.db.C(CollectionIngress)
	err := collection.Update(ingress.IngressResource{
		Ingress: model.Ingress{
			Name: name,
		},
		NamespaceID: namespaceID,
	}.OneSelectQuery(),
		bson.M{
			"$set": bson.M{"deleted": true},
		})
	if err != nil {
		mongo.logger.WithError(err).Errorf("unable to delete ingress")
		if err == mgo.ErrNotFound {
			return rserrors.ErrResourceNotExists().AddDetails(name)
		}
		return PipErr{err}.ToMongerr().Extract()
	}
	return nil
}

func (mongo *MongoStorage) RestoreIngress(namespaceID, name string) error {
	mongo.logger.Debugf("restoring ingress")
	var collection = mongo.db.C(CollectionIngress)
	err := collection.Update(ingress.IngressResource{
		Ingress: model.Ingress{
			Name: name,
		},
		NamespaceID: namespaceID,
	}.OneSelectDeletedQuery(),
		bson.M{
			"$set": bson.M{"deleted": false},
		})
	if err != nil {
		mongo.logger.WithError(err).Errorf("unable to restore ingress")
		if err == mgo.ErrNotFound {
			return rserrors.ErrResourceNotExists().AddDetails(name)
		}
		return PipErr{err}.ToMongerr().Extract()
	}
	return nil
}

func (mongo *MongoStorage) DeleteAllIngressesInNamespace(namespace string) error {
	mongo.logger.Debugf("deleting all ingresses in namespace")
	var collection = mongo.db.C(CollectionIngress)
	_, err := collection.UpdateAll(ingress.IngressResource{
		NamespaceID: namespace,
	}.AllSelectQuery(),
		bson.M{
			"$set": bson.M{"deleted": true},
		})
	if err != nil {
		mongo.logger.WithError(err).Errorf("unable to delete deployment")
	}
	return PipErr{err}.ToMongerr().Extract()
}

func (mongo *MongoStorage) DeleteAllIngressesByOwner(owner string) error {
	mongo.logger.Debugf("deleting all user ingresses")
	var collection = mongo.db.C(CollectionIngress)
	_, err := collection.UpdateAll(ingress.IngressResource{
		Ingress: model.Ingress{Owner: owner},
	}.AllSelectOwnerQuery(),
		bson.M{
			"$set": bson.M{"deleted": true},
		})
	if err != nil {
		mongo.logger.WithError(err).Errorf("unable to delete deployments")
	}
	return PipErr{err}.ToMongerr().Extract()
}

func (mongo *MongoStorage) CountIngresses(owner string) (int, error) {
	mongo.logger.Debugf("counting ingresses")
	var collection = mongo.db.C(CollectionIngress)
	if n, err := collection.Find(bson.M{
		"ingress.owner": owner,
		"deleted":       false,
	}).Count(); err != nil {
		return 0, PipErr{err}.ToMongerr().NotFoundToNil().Extract()
	} else {
		return n, nil
	}
}

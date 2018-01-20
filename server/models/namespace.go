package models

import (
	"database/sql"
	"time"

	rstypes "git.containerum.net/ch/json-types/resource-service"
	rserrors "git.containerum.net/ch/resource-service/server/errors"

	"context"

	"git.containerum.net/ch/utils"
)

func (db ResourceSvcDB) NamespaceCreate(ctx context.Context, tariff rstypes.NamespaceTariff, user string, label string) (nsID string, err error) {
	nsID = utils.NewUUID()
	{
		var count int
		err = db.qLog.QueryRowxContext(ctx, `SELECT count(*)
									FROM accesses
									WHERE user_id=$1 AND resource_label=$2 AND kind='Namespace'`,
			user, label).Scan(&count)
		if err != nil {
			return
		}
		if count > 0 {
			err = rserrors.ErrAlreadyExists
			return
		}
	}

	_, err = db.eLog.ExecContext(ctx,
		`INSERT INTO namespaces (
			id,
			ram,
			cpu,
			max_ext_svc,
			max_int_svc,
			max_traffic,
			tariff_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		nsID,
		tariff.MemoryLimit,
		tariff.CpuLimit,
		tariff.ExternalServices,
		tariff.InternalServices,
		tariff.Traffic,
		tariff.TariffID,
	)
	if err != nil {
		return
	}

	_, err = db.eLog.ExecContext(ctx,
		`INSERT INTO accesses(
			id,
			kind,
			resource_id,
			resource_label,
			user_id,
			owner_user_id,
			access_level,
			access_level_change_time,
			limited,
			new_access_level
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		utils.NewUUID(),
		"Namespace",
		nsID,
		label,
		user,
		user,
		"owner",
		time.Now(),
		false,
		"owner",
	)
	if err != nil {
		return
	}

	return
}

func (db ResourceSvcDB) NamespaceList(ctx context.Context, user string) (nss []rstypes.Namespace, err error) {
	rows, err := db.qLog.QueryContext(ctx,
		`SELECT
			n.id,
			n.create_time,
			n.deleted,
			n.delete_time,
			n.tariff_id,
			a.resource_label,
			a.access_level,
			a.access_level_change_time,
			a.limited,
			a.new_access_level,
			n.ram,
			n.cpu,
			n.max_ext_svc,
			n.max_int_svc,
			n.max_traffic
		FROM namespaces n INNER JOIN accesses a ON a.resource_id=n.id
		WHERE a.user_id=$1 AND a.kind='Namespace'`,
		user,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var ns rstypes.Namespace
		err = rows.Scan(
			&ns.ID,
			&ns.CreateTime,
			&ns.Deleted,
			&ns.DeleteTime,
			&ns.TariffID,
			&ns.Label,
			&ns.Access,
			&ns.AccessChangeTime,
			&ns.Limited,
			&ns.NewAccess,
			&ns.RAM,
			&ns.CPU,
			&ns.MaxExtService,
			&ns.MaxIntService,
			&ns.MaxTraffic,
		)
		if err != nil {
			return
		}
		nss = append(nss, ns)
	}
	return
}

func (db ResourceSvcDB) NamespaceRename(ctx context.Context, user string, oldname, newname string) (err error) {
	_, err = db.eLog.ExecContext(ctx,
		`UPDATE accesses SET resource_label=$1
		WHERE resource_label=$2 AND user_id=$3 AND kind='Namespace'`,
		newname,
		oldname,
		user,
	)
	return
}

func (db ResourceSvcDB) NamespaceSetLimited(ctx context.Context, owner string, ownerLabel string, limited bool) (err error) {
	_, err = db.eLog.ExecContext(ctx,
		`UPDATE accesses SET limited=$3
		WHERE user_id=$1 AND resource_label=$2 AND kind='Namespace'`,
		owner,
		ownerLabel,
		limited,
	)
	return
}

func (db ResourceSvcDB) NamespaceSetAccess(ctx context.Context, owner string, label string, other string, access string) (err error) {
	var resID string

	// get resource id
	err = db.qLog.QueryRowxContext(ctx,
		`SELECT resource_id FROM accesses
		WHERE user_id=$1 AND resource_label=$2 AND owner_user_id=user_id AND kind='Namespace'`,
		owner,
		label,
	).Scan(&resID)
	switch err {
	case nil:
	case sql.ErrNoRows:
		return rserrors.ErrNoSuchResource
	default:
		return
	}

	if other == owner {
		_, err = db.eLog.ExecContext(ctx,
			`UPDATE accesses SET new_access_level=$1
			WHERE owner_user_id=$2 AND resource_id=$3 AND kind='Namespace'`,
			access,
			owner,
			resID,
		)
	} else {
		_, err = db.eLog.ExecContext(ctx,
			`INSERT INTO accesses (
					id,
					kind,
					resource_id,
					resouce_label,
					user_id,
					owner_user_id,
					access_level,
					new_access_level
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
				ON CONFLICT (resource_id, user_id) DO UPDATE SET new_access_level = $8`,
			utils.NewUUID(),
			"Namespace",
			resID,
			utils.NewUUID(),
			other,
			owner,
			access,
			access,
		)
	}

	return
}

func (db ResourceSvcDB) NamespaceSetTariff(ctx context.Context, owner string, label string, t rstypes.NamespaceTariff) (err error) {
	res, err := db.eLog.ExecContext(ctx,
		`
		WITH owner_ns_id AS (
			SELECT resource_id FROM accesses
			WHERE owner_user_id=user_id 
					AND user_id=$1 
					AND resource_label=$2
					AND kind='Namespace'
		)
		UPDATE namespaces SET
			tariff_id=$3,
			cpu=$4,
			ram=$5,
			max_traffic=$6,
			max_ext_svc=$7,
			max_int_svc=$8
		WHERE id IN (SELECT * FROM owner_ns_id)`,
		owner,
		label,
		t.TariffID,
		t.CpuLimit,
		t.MemoryLimit,
		t.Traffic,
		t.ExternalServices,
		t.InternalServices,
	)
	if err != nil {
		return
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return
	}
	if affected == 0 { // means namespace is not exists
		err = rserrors.ErrNoSuchResource
	}

	return
}

func (db ResourceSvcDB) NamespaceDelete(ctx context.Context, user string, label string) (err error) {
	var accessLevel string
	var owner string
	var resID string
	var subVolsCnt int

	err = db.qLog.QueryRowxContext(ctx,
		`SELECT a.access_level, a.owner_user_id, a.resource_id, count(nv.*) as vol_count
				FROM accesses a
				JOIN namespace_volume nv ON nv.ns_id = a.resource_id
				WHERE user_id=$1 AND resource_label=$2 AND kind='Namespace'
				GROUP BY a.access_level, a.owner_user_id, a.resource_id`,
		user, label).Scan(&accessLevel, &owner, &resID, &subVolsCnt)
	switch err {
	case nil:
	case sql.ErrNoRows:
		return rserrors.ErrNoSuchResource
	default:
		return
	}

	if owner == user && subVolsCnt > 0 {
		err = rserrors.NewPermissionError("cannot delete, namespace has associated volumes")
		return
	}

	_, err = db.eLog.ExecContext(ctx,
		`UPDATE namespaces
		SET deleted=true, delete_time=statement_timestamp()
		WHERE id=$1 AND $2=$3`, // if user is owner of namespace set it as deleted
		resID, owner, user)
	if err != nil {
		return err
	}

	_, err = db.eLog.ExecContext(ctx, `
		DELETE FROM accesses 
			WHERE resource_id=$1 
				AND (user_id=$3 OR $2=$3)`, // if user is owner of namespace all accesses will be removed, if not - only user accesses to ns
		resID, owner, user)

	return
}

func (db ResourceSvcDB) NamespaceAccesses(ctx context.Context, owner string, label string) (ns rstypes.Namespace, err error) {
	err = db.qLog.QueryRowxContext(ctx,
		`SELECT
			n.id,
			n.create_time,
			n.deleted,
			n.delete_time,
			a.user_id,
			n.tariff_id,
			a.resource_label,
			a.access_level,
			a.access_level_change_time,
			n.ram,
			n.cpu,
			n.max_ext_svc,
			n.max_int_svc,
			n.max_traffic
		FROM accesses a INNER JOIN namespaces n ON n.id=a.resource_id
		WHERE a.user_id=$1 AND a.resource_label=$2 AND a.owner_user_id=a.user_id AND a.kind='Namespace'`,
		owner,
		label,
	).Scan(
		&ns.ID,
		&ns.CreateTime,
		&ns.Deleted,
		&ns.DeleteTime,
		&ns.UserID,
		&ns.TariffID,
		&ns.Label,
		&ns.Access,
		&ns.AccessChangeTime,
		&ns.RAM,
		&ns.CPU,
		&ns.MaxExtService,
		&ns.MaxIntService,
		&ns.MaxTraffic,
	)
	switch err {
	case nil:
	case sql.ErrNoRows:
		err = rserrors.ErrNoSuchResource
		return
	default:
		return
	}

	rows, err := db.qLog.QueryxContext(ctx,
		`SELECT
			user_id,
			access_level,
			limited,
			new_access_level,
			access_level_change_time
		FROM accesses
		WHERE kind='Namespace' AND resource_id=$1`,
		ns.ID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var ar rstypes.AccessRecord
		err = rows.StructScan(&rows)
		if err != nil {
			return
		}
		ns.Users = append(ns.Users, ar)
	}
	return
}

func (db ResourceSvcDB) NamespaceVolumeAssociate(ctx context.Context, nsID, vID string) (err error) {
	_, err = db.eLog.ExecContext(ctx,
		`INSERT INTO namespace_volume (ns_id, vol_id)
		VALUES ($1,$2)`,
		nsID,
		vID,
	)
	return
}

func (db ResourceSvcDB) NamespaceVolumeListAssoc(ctx context.Context, nsID string) (vl []Volume, err error) {
	rows, err := db.qLog.QueryContext(ctx,
		`SELECT nv.vol_id,
			v.create_time,
			v.deleted,
			v.delete_time,
			v.tariff_id,
			a.resource_label,
			a.access_level,
			a.access_level_change_time,
			a.limited,
			a.new_access_level,
			v.capacity,
			v.replicas,
			v.is_persistent
		FROM namespace_volume nv
		INNER JOIN accesses a ON a.resource_id = nv.vol_id
		INNER JOIN volumes v ON v.id = nv.vol_id
		WHERE nv.ns_id=$1`,
		nsID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var v Volume
		err = rows.Scan(
			&v.ID,
			&v.CreateTime,
			&v.Deleted,
			&v.DeleteTime,
			&v.TariffID,
			&v.Label,
			&v.Access,
			&v.AccessChangeTime,
			&v.Limited,
			&v.NewAccess,
			&v.Storage,
			&v.Replicas,
			&v.Persistent,
		)
		if err != nil {
			return
		}
		vl = append(vl, v)
	}
	return
}

func (db ResourceSvcDB) NamespacesDeleteAll(ctx context.Context, owner string) error {
	_, err := db.eLog.ExecContext(ctx, `
		WITH resources_to_update AS (
			SELECT resource_id FROM accesses
			WHERE user_id = $1 AND kind = 'Namespace'
		)
		UPDATE namespaces
		SET deleted=true, delete_time=statement_timestamp()
		WHERE id IN (SELECT * FROM resources_to_update)`, owner)
	return err
}

// Copyright © 2018 Prateek Malhotra <someone1@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Based on https://github.com/ory/hydra/blob/master/warden/group/manager_sql.go

package group

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	wgroup "github.com/ory/hydra/warden/group"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

const (
	wardenGroupKind    = "HydraWardenGroup"
	wardenGroupVersion = 1
)

//DatastoreManager implements group.Manager using Google's Datastore
type DatastoreManager struct {
	client    *datastore.Client
	context   context.Context
	namespace string
}

// NewDatastoreManager will initialize and return a DatastoreManager.
func NewDatastoreManager(ctx context.Context, client *datastore.Client, namespace string) *DatastoreManager {
	return &DatastoreManager{
		client:    client,
		context:   ctx,
		namespace: namespace,
	}
}

type hydraWardenGroup struct {
	Version int            `datastore:"v"`
	ID      string         `datastore:"-"`
	Members []string       `datastore:"m"`
	Key     *datastore.Key `datastore:"-"`

	update bool
}

func (h *hydraWardenGroup) LoadKey(k *datastore.Key) error {
	h.Key = k
	h.ID = k.Name

	return nil
}

func (h *hydraWardenGroup) Load(ps []datastore.Property) error {
	err := datastore.LoadStruct(h, ps)
	if _, ok := err.(*datastore.ErrFieldMismatch); err != nil && !ok {
		return errors.WithStack(err)
	}

	switch h.Version {
	case wardenGroupVersion:
		// Up to date, nothing to do
		break
	// case 1:
	// 	// Update to version 2 here
	// 	fallthrough
	// case 2:
	// 	//update to version 3 here
	// 	fallthrough
	case -1:
		// This is here to complete saving the entity should we need to udpate it
		if h.Version == -1 {
			return errors.New(fmt.Sprintf("unexpectedly got to version update trigger with incorrect version -1"))
		}
		h.Version = wardenGroupVersion
		h.update = true
	default:
		return errors.New(fmt.Sprintf("got unexpected version %d when loading entity", h.Version))
	}
	return nil
}

func (h *hydraWardenGroup) Save() ([]datastore.Property, error) {
	h.Version = wardenGroupVersion
	return datastore.SaveStruct(h)
}

func (d *DatastoreManager) createGroupKey(groupID string) *datastore.Key {
	key := datastore.NameKey(wardenGroupKind, groupID, nil)
	key.Namespace = d.namespace
	return key
}

func (d *DatastoreManager) CreateGroup(g *wgroup.Group) error {
	if g.ID == "" {
		g.ID = uuid.New()
	}

	key := d.createGroupKey(g.ID)
	group := &hydraWardenGroup{
		Members: g.Members,
	}

	mutation := datastore.NewInsert(key, group)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) GetGroup(id string) (*wgroup.Group, error) {
	var found hydraWardenGroup
	key := d.createGroupKey(id)

	if err := d.client.Get(d.context, key, &found); err != nil {
		return nil, errors.WithStack(err)
	}

	if found.update {
		mutation := datastore.NewUpdate(key, &found)
		if _, err := d.client.Mutate(d.context, mutation); err != nil {
			return nil, errors.WithStack(err)
		}
		found.update = false
	}

	return &wgroup.Group{
		ID:      found.ID,
		Members: found.Members,
	}, nil
}

func (d *DatastoreManager) DeleteGroup(id string) error {
	key := d.createGroupKey(id)

	if err := d.client.Delete(d.context, key); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) AddGroupMembers(groupID string, subjects []string) error {
	key := d.createGroupKey(groupID)

	_, err := d.client.RunInTransaction(d.context, func(tx *datastore.Transaction) error {
		var group hydraWardenGroup
		if err := tx.Get(key, &group); err != nil {
			return errors.WithStack(err)
		}

		for _, subject := range subjects {
			for _, exists := range group.Members {
				if subject == exists {
					return errors.New("duplicate group entry")
				}
			}
			group.Members = append(group.Members, subject)
		}

		mutation := datastore.NewUpdate(key, &group)
		if _, err := tx.Mutate(mutation); err != nil {
			return errors.WithStack(err)
		}
		return nil
	})

	if err != nil {
		return errors.Wrap(err, "Could not commit transaction")
	}

	return nil
}

func (d *DatastoreManager) RemoveGroupMembers(groupID string, subjects []string) error {
	key := d.createGroupKey(groupID)

	_, err := d.client.RunInTransaction(d.context, func(tx *datastore.Transaction) error {
		var group hydraWardenGroup
		if err := tx.Get(key, &group); err != nil {
			return errors.WithStack(err)
		}
		updated := false
		for _, subject := range subjects {
			for idx := range group.Members {
				if subject == group.Members[idx] {
					group.Members = append(group.Members[:idx], group.Members[idx+1:]...)
					updated = true
					break
				}
			}
		}

		if updated {
			mutation := datastore.NewUpdate(key, &group)
			if _, err := tx.Mutate(mutation); err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	})

	if err != nil {
		return errors.Wrap(err, "Could not commit transaction")
	}

	return nil
}

func (d *DatastoreManager) FindGroupsByMember(subject string, limit, offset int) ([]wgroup.Group, error) {
	query := datastore.NewQuery(wardenGroupKind).Filter("m =", subject).Limit(limit).Offset(offset).Namespace(d.namespace)
	return d.executeQuery(query)
}

func (d *DatastoreManager) ListGroups(limit, offset int) ([]wgroup.Group, error) {
	query := datastore.NewQuery(wardenGroupKind).Limit(limit).Offset(offset).Namespace(d.namespace)
	return d.executeQuery(query)
}

func (d *DatastoreManager) executeQuery(query *datastore.Query) ([]wgroup.Group, error) {
	var groups []hydraWardenGroup

	if _, err := d.client.GetAll(d.context, query, &groups); err != nil {
		return nil, errors.WithStack(err)
	}

	// Let's check if we need to update any groups
	var mutations []*datastore.Mutation
	for idx := range groups {
		if groups[idx].update {
			mutations = append(mutations, datastore.NewUpdate(groups[idx].Key, &groups[idx]))
			groups[idx].update = false
		}
	}
	if len(mutations) > 0 {
		if _, err := d.client.Mutate(d.context, mutations...); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	var wgroups = make([]wgroup.Group, len(groups))
	for idx, group := range groups {
		wgroups[idx] = wgroup.Group{
			ID:      group.ID,
			Members: group.Members,
		}
	}

	return wgroups, nil
}

func typecheck() {
	var _ wgroup.Manager = (*DatastoreManager)(nil)
}

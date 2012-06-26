package mstate

import (
	"errors"
	"fmt"
	"labix.org/v2/mgo/bson"
	"launchpad.net/juju-core/charm"
	"strconv"
)

// Service represents the state of a service.
type Service struct {
	st   *State
	name string
}

// serviceDoc represents the internal state of a service in MongoDB.
type serviceDoc struct {
	Name     string `bson:"_id"`
	CharmURL *charm.URL
}

// Name returns the service name.
func (s *Service) Name() string {
	return s.name
}

// CharmURL returns the charm URL this service is supposed to use.
func (s *Service) CharmURL() (url *charm.URL, err error) {
	sdoc := &serviceDoc{}
	err = s.st.services.Find(bson.D{{"_id", s.name}}).One(sdoc)
	if err != nil {
		return nil, fmt.Errorf("can't get the charm URL of service %q: %v", s, err)
	}
	return sdoc.CharmURL, nil
}

// SetCharmURL changes the charm URL for the service.
func (s *Service) SetCharmURL(url *charm.URL) (err error) {
	change := bson.D{{"$set", bson.D{{"charmurl", url}}}}
	err = s.st.services.Update(bson.D{{"_id", s.name}}, change)
	if err != nil {
		return fmt.Errorf("can't set the charm URL of service %q: %v", s, err)
	}
	return nil
}

// Charm returns the service's charm.
func (s *Service) Charm() (*Charm, error) {
	url, err := s.CharmURL()
	if err != nil {
		return nil, err
	}
	return s.st.Charm(url)
}

// String returns the service name.
func (s *Service) String() string {
	return s.Name()
}

// addUnit adds a new unit to the service. If s is a subordinate service,
// principalNanem must be the unit key of some principal unit.
func (s *Service) addUnit(principalName string) (unit *Unit, err error) {
	defer errorContextf(&err, "can't add unit to service %q", s)
	id, err := s.st.sequence(s.Name())
	if err != nil {
		return nil, err
	}
	name := s.name + "/" + strconv.Itoa(id)
	udoc := unitDoc{
		Name:        name,
		ServiceName: s.name,
		IsPrincipal: principalName == "",
	}
	err = s.st.units.Insert(udoc)
	if err != nil {
		return nil, err
	}
	return &Unit{
		st:          s.st,
		name:        name,
		serviceName: s.name,
		isPrincipal: principalName == "",
	}, nil
}

// AddUnit adds a new principal unit to the service.
func (s *Service) AddUnit() (unit *Unit, err error) {
	ch, err := s.Charm()
	if err != nil {
		return nil, fmt.Errorf("can't add unit to service %q: %v", err)
	}
	if ch.Meta().Subordinate {
		return nil, fmt.Errorf("cannot directly add units to subordinate service %q", s)
	}
	return s.addUnit("")
}

// AddUnitSubordinateTo adds a new subordinate unit to the service,
// subordinate to principal.
func (s *Service) AddUnitSubordinateTo(principal *Unit) (*Unit, error) {
	ch, err := s.Charm()
	if err != nil {
		return nil, fmt.Errorf("can't add unit to service %q: %v", err)
	}
	if !ch.Meta().Subordinate {
		return nil, fmt.Errorf("can't add unit of principal service %q as a subordinate of %q", s, principal)
	}
	if !principal.IsPrincipal() {
		return nil, errors.New("a subordinate unit must be added to a principal unit")
	}
	return s.addUnit(principal.name)
}

// RemoveUnit() removes a unit.
func (s *Service) RemoveUnit(unit *Unit) error {
	sel := bson.D{
		{"_id", unit.name},
		{"servicename", s.name},
	}
	err := s.st.units.Remove(sel)
	if err != nil {
		return fmt.Errorf("can't remove unit %q: %v", unit, err)
	}
	// TODO unassign from machine if currently assigned.
	return nil
}

// Unit returns the service's unit with name.
func (s *Service) Unit(name string) (*Unit, error) {
	udoc := &unitDoc{}
	sel := bson.D{
		{"_id", name},
		{"servicename", s.name},
	}
	err := s.st.units.Find(sel).One(udoc)
	if err != nil {
		return nil, fmt.Errorf("can't get unit %q from service %q: %v", name, s.name, err)
	}
	return &Unit{
		st:          s.st,
		name:        name,
		serviceName: s.name,
		isPrincipal: udoc.IsPrincipal,
	}, nil
}

// AllUnits returns all units of the service.
func (s *Service) AllUnits() (units []*Unit, err error) {
	udocs := []unitDoc{}
	err = s.st.units.Find(bson.D{{"servicename", s.name}}).All(&udocs)
	if err != nil {
		return nil, fmt.Errorf("can't get all units from service %q: %v", err)
	}
	for _, v := range udocs {
		unit := &Unit{
			st:          s.st,
			name:        v.Name,
			serviceName: s.name,
			isPrincipal: v.IsPrincipal,
		}
		units = append(units, unit)
	}
	return units, nil
}

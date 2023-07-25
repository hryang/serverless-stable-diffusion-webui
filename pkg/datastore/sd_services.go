package datastore

import (
	"fmt"
)

const kSDServicesTableName = "stable_diffusion_services"
const kSDServiceNameColumnName = "SERVICE_NAME"
const kSDServiceEndpointColumnName = "SERVICE_ENDPOINT"

// SDServiceEndpoint is the backend stable diffusion service endpoint.
type SDServiceEndpoint struct {
	Name     string
	Endpoint string
}

// SDServices datastore stores the stable-diffusion backend services' endpoints.
type SDServices struct {
	ds Datastore
}

// NewSDServices create stable-diffusion services datastore.
func NewSDServices(dbType DatastoreType, dbName string) (*SDServices, error) {
	config := &Config{
		Type:      dbType,
		DBName:    dbName,
		TableName: kSDServicesTableName,
		ColumnConfig: map[string]string{
			kSDServiceNameColumnName:     "text primary key not null",
			kSDServiceEndpointColumnName: "text",
		},
		PrimaryKeyColumnName: kSDServiceNameColumnName,
	}
	df := DatastoreFactory{}
	ds, err := df.New(config)
	if err != nil {
		return nil, err
	}
	s := &SDServices{
		ds: ds,
	}
	return s, nil
}

func (s *SDServices) Close() error {
	return s.ds.Close()
}

// PutServiceEndpoint put the service endpoint of the specified model to the underlying datastore.
func (s *SDServices) PutServiceEndpoint(serviceName string, endpoint string) error {
	if serviceName == "" {
		return fmt.Errorf("task id cannot be empty")
	}
	err := s.ds.Put(serviceName, map[string]interface{}{
		kSDServiceEndpointColumnName: endpoint,
	})
	return err
}

// GetServiceEndpoint get the service endpoint of the specified model from the underlying datastore.
func (s *SDServices) GetServiceEndpoint(serviceName string) (string, error) {
	result, err := s.ds.Get(serviceName, []string{kSDServiceEndpointColumnName})
	if err != nil {
		return "", err
	}
	val := result[kSDServiceEndpointColumnName]
	if val == nil {
		return "", nil // return empty string for non-existent task
	}
	return val.(string), nil
}

// ListAllServiceEndpoints return all the service endpoints as an array of [service_name, service_endpoint].
func (s *SDServices) ListAllServiceEndpoints() ([]SDServiceEndpoint, error) {
	result, err := s.ds.ListAll()
	if err != nil {
		return nil, err
	}

	var ret []SDServiceEndpoint
	for k, v := range result {
		serviceName := k
		serviceEndpoint := v[kSDServiceEndpointColumnName].(string)
		ret = append(ret, SDServiceEndpoint{
			Name:     serviceName,
			Endpoint: serviceEndpoint})
	}

	return ret, nil
}

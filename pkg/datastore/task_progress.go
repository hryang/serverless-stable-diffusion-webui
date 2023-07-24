package datastore

import "fmt"

const kTaskProgressTableName = "task_progress"
const kTaskIdColumnName = "TASK_ID"
const kTaskProgressColumnName = "TASK_PROGRESS"

// TaskProgress read/write the task progress to the underlying datastore.
type TaskProgress struct {
	ds Datastore
}

func NewTaskProgress(dbType DatastoreType, dbName string) (*TaskProgress, error) {
	config := &Config{
		Type:      dbType,
		DBName:    dbName,
		TableName: kTaskProgressTableName,
		ColumnConfig: map[string]string{
			kTaskIdColumnName:       "text primary key not null",
			kTaskProgressColumnName: "text",
		},
		PrimaryKeyColumnName: kTaskIdColumnName,
	}
	df := DatastoreFactory{}
	ds, err := df.New(config)
	if err != nil {
		return nil, err
	}
	t := &TaskProgress{
		ds: ds,
	}
	return t, nil
}

// Close close the underlying datastore.
func (t *TaskProgress) Close() error {
	return t.ds.Close()
}

// PutProgress persist the task progress to the underlying datastore.
func (t *TaskProgress) PutProgress(taskId string, serializedProgress string) error {
	if taskId == "" {
		return fmt.Errorf("task id cannot be empty")
	}
	err := t.ds.Put(taskId, map[string]interface{}{
		kTaskProgressColumnName: serializedProgress,
	})
	return err
}

// GetProgress get the specified task progress from the underlying datastore,
// and return the result as json serialized string.
func (t *TaskProgress) GetProgress(taskId string) (string, error) {
	result, err := t.ds.Get(taskId, []string{kTaskProgressColumnName})
	if err != nil {
		return "", err
	}
	val := result[kTaskProgressColumnName]
	if val == nil {
		return "", nil // return empty string for non-existent task
	}
	return val.(string), nil
}

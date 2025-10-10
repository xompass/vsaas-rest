package database

import (
	"github.com/go-errors/errors"
)

// Connector es una interfaz genérica para cualquier tipo de conector de base de datos
type Connector interface {
	Ping() error
	Disconnect() error
	GetName() string
	GetDatabaseName() string
	GetDriver() any
}

type Datasource struct {
	connectors           map[string]Connector // Connectors registered in the datasource. This allows to have multiple connectors for different databases.
	repositories         map[string]any       // Repositories registered in the datasource.
	models               map[string]IModel    // Models registered in the datasource.
	connectorByModelName map[string]Connector // Connectors by model name.
}

func (receiver *Datasource) AddConnector(connector Connector) error {
	if receiver == nil {
		return errors.New("datasource is nil")
	}

	if receiver.connectors == nil {
		receiver.connectors = make(map[string]Connector)
	}

	receiver.connectors[connector.GetName()] = connector
	return nil
}

func (receiver *Datasource) Destroy() {
	for _, connector := range receiver.connectors {
		if connector != nil {
			_ = connector.Disconnect()
		}
	}
}

func (receiver *Datasource) RegisterModel(model IModel) error {
	if receiver == nil {
		return errors.New("datasource is nil")
	}

	connectorName := model.GetConnectorName()
	modelName := model.GetModelName()
	connector, err := receiver.GetConnector(connectorName)
	if err != nil {
		return err
	}

	if receiver.models == nil {
		receiver.models = make(map[string]IModel)
	}

	if receiver.connectorByModelName == nil {
		receiver.connectorByModelName = make(map[string]Connector)
	}

	if receiver.connectorByModelName[modelName] != nil {
		return errors.Errorf("the model %s is already registered with connector %s", modelName, receiver.connectorByModelName[modelName].GetName())
	}

	receiver.models[modelName] = model
	receiver.connectorByModelName[modelName] = connector
	return nil
}

func (receiver *Datasource) GetModelConnector(model IModel) (Connector, error) {
	if receiver == nil {
		return nil, errors.New("datasource is nil")
	}

	connector, ok := receiver.connectorByModelName[model.GetModelName()]
	if !ok {
		return nil, errors.Errorf("the model %s is not registered", model.GetModelName())
	}

	return connector, nil
}

func (receiver *Datasource) GetConnector(name string) (Connector, error) {
	if receiver == nil {
		return nil, errors.New("datasource is nil")
	}

	connector, ok := receiver.connectors[name]
	if !ok {
		return nil, errors.Errorf("the connector %s is not registered", name)
	}

	return connector, nil
}

func (receiver *Datasource) GetModel(modelName string) (IModel, error) {
	if receiver == nil {
		return nil, errors.New("datasource is nil")
	}

	if receiver.models == nil {
		return nil, errors.New("no models registered in the datasource")
	}

	model, ok := receiver.models[modelName]
	if !ok {
		return nil, errors.Errorf("the model %s is not registered", modelName)
	}

	return model, nil
}

func RegisterDatasourceRepository[T IModel](ds *Datasource, model T, repository Repository[T]) error {
	if ds == nil || repository == nil {
		return errors.New("datasource or repository cannot be nil")
	}

	modelName := model.GetModelName()

	if ds.repositories == nil {
		ds.repositories = make(map[string]any)
	}

	repositoryConnector := repository.GetConnector()
	if repositoryConnector == nil {
		return errors.Errorf("repository for model %s does not have a connector", modelName)
	}

	connectorExists := false
	for _, existingConnector := range ds.connectors {
		if existingConnector == repositoryConnector {
			connectorExists = true
			break
		}
	}
	if !connectorExists {
		return errors.Errorf("the connector %s for model %s is not registered in the datasource", repositoryConnector.GetName(), modelName)
	}

	repositoryExists := false
	currentModelName := ""
	for modelName, existingRepository := range ds.repositories {
		if existingRepository == repository {
			repositoryExists = true
			currentModelName = modelName
			break
		}
	}

	if repositoryExists {
		return errors.Errorf("the repository is already registered for model %s, current model name is %s", modelName, currentModelName)
	}

	ds.repositories[modelName] = repository

	return nil
}

func GetDatasourceModelRepository[T IModel](datasource *Datasource, model T) (Repository[T], error) {
	if datasource == nil {
		return nil, errors.New("datasource is nil")
	}

	repository, ok := datasource.repositories[model.GetModelName()]
	if !ok {
		return nil, errors.Errorf("the model %s is not registered", model.GetModelName())
	}

	if repo, ok := repository.(Repository[T]); ok {
		return repo, nil
	}

	return nil, errors.Errorf("the repository for model %s is not of the expected type", model.GetModelName())
}

/**
 * EnsureIndexes ensures that all indexes defined in registered models are created.
 * This method should be called after all models are registered.
 * It will delegate to the appropriate IndexManager for each connector type.
 */
func (receiver *Datasource) EnsureIndexes() error {
	if receiver == nil {
		return errors.New("datasource is nil")
	}

	if len(receiver.models) == 0 {
		return nil // No models to process
	}

	for modelName, model := range receiver.models {
		connector, err := receiver.GetModelConnector(model)
		if err != nil {
			return errors.Errorf("failed to get connector for model %s: %v", modelName, err)
		}

		// Check if connector is MongoDB
		if mongoConnector, ok := connector.(*MongoConnector); ok {
			indexManager := mongoConnector.GetIndexManager()
			if indexManager != nil {
				if err := indexManager.EnsureIndexes(model); err != nil {
					return errors.Errorf("failed to ensure indexes for model %s: %v", modelName, err)
				}
			}
		}
		// Future: Add support for other database types here
		// else if postgresConnector, ok := connector.(*PostgresConnector); ok {
		//     indexManager := postgresConnector.GetIndexManager()
		//     if err := indexManager.EnsureIndexes(model); err != nil {
		//         return err
		//     }
		// }
	}

	return nil
}

/**
 * EnsureIndexesForModel ensures indexes for a specific model.
 * This is useful when you want to ensure indexes for a single model
 * instead of all registered models.
 */
func (receiver *Datasource) EnsureIndexesForModel(model IModel) error {
	if receiver == nil {
		return errors.New("datasource is nil")
	}

	connector, err := receiver.GetModelConnector(model)
	if err != nil {
		return errors.Errorf("failed to get connector for model %s: %v", model.GetModelName(), err)
	}

	// Check if connector is MongoDB
	if mongoConnector, ok := connector.(*MongoConnector); ok {
		indexManager := mongoConnector.GetIndexManager()
		if indexManager != nil {
			return indexManager.EnsureIndexes(model)
		}
	}
	// Future: Add support for other database types

	return nil
}

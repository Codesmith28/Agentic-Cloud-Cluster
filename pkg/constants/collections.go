package constants

// MongoDB collection names across Agentic Cloud Cluster.
const (
	CollectionTasks                 = "TASKS"
	CollectionWorkers               = "WORKER_REGISTRY"
	CollectionAssignments           = "ASSIGNMENTS"
	CollectionAttempts              = "ATTEMPTS"
	CollectionResults               = "RESULTS"
	CollectionUsers                 = "USERS"
	CollectionSchedulerModels       = "SCHEDULER_MODELS"
	CollectionSchedulerModelsFiles  = "scheduler_models.files"
	CollectionSchedulerModelsChunks = "scheduler_models.chunks"
	CollectionRTSWeights            = "RTS_WEIGHTS"
	CollectionFileMetadata          = "FILE_METADATA"
	CollectionFiles                 = "FILES"
)

// AllRequiredCollections lists all collections that should be ensured at startup.
var AllRequiredCollections = []string{
	CollectionUsers,
	CollectionWorkers,
	CollectionTasks,
	CollectionAssignments,
	CollectionAttempts,
	CollectionResults,
	CollectionSchedulerModels,
	CollectionSchedulerModelsFiles,
	CollectionSchedulerModelsChunks,
	CollectionRTSWeights,
	CollectionFileMetadata,
}

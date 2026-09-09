package tasks

import "barbtils/internal/database"

// The full vocabulary of each enum, in the order the schema declares them, so
// the CLI and the TUI offer exactly what the database accepts.

func AllTypes() []database.TasksTypes {
	return []database.TasksTypes{
		database.TasksTypesOneTime,
		database.TasksTypesRecurring,
	}
}

func AllPriorities() []database.TasksPriorities {
	return []database.TasksPriorities{
		database.TasksPrioritiesUnknown,
		database.TasksPrioritiesVeryLow,
		database.TasksPrioritiesLow,
		database.TasksPrioritiesMedium,
		database.TasksPrioritiesHigh,
		database.TasksPrioritiesVeryHigh,
	}
}

func AllCategories() []database.TasksCategories {
	return []database.TasksCategories{
		database.TasksCategoriesUnknown,
		database.TasksCategoriesClientProject,
		database.TasksCategoriesPersonalProject,
		database.TasksCategoriesTroubleshooting,
		database.TasksCategoriesRoutine,
	}
}

func AllStatuses() []database.TasksStatuses {
	return []database.TasksStatuses{
		database.TasksStatusesPending,
		database.TasksStatusesInProgress,
		database.TasksStatusesPaused,
		database.TasksStatusesCompleted,
		database.TasksStatusesArchived,
	}
}

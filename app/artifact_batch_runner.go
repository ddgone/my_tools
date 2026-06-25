package main

func recountArtifactTask(task *ArtifactBatchTask) {
	task.TotalCount = len(task.Items)
	task.SuccessCount = 0
	task.ErrorCount = 0
	task.CachedCount = 0
	task.SkippedCount = 0
	for _, item := range task.Items {
		switch item.Status {
		case "success":
			task.SuccessCount++
		case "error":
			task.ErrorCount++
		case "cached":
			task.CachedCount++
		case "skipped":
			task.SkippedCount++
		}
	}
}

func cloneArtifactTask(task *ArtifactBatchTask) *ArtifactBatchTask {
	if task == nil {
		return nil
	}
	copyTask := *task
	copyTask.Items = append([]ArtifactBatchItemResult(nil), task.Items...)
	return &copyTask
}

# Jobs

Jobs are typed background adapters under `app/jobs`. A job decodes its payload,
loads services from context, and invokes one service operation. Business rules
and durable state transitions stay in services.

```go
// app/jobs/basejob/basejob.go
type BaseJob struct {
	lazyjobs.BaseJob
}
```

```go
// app/jobs/send_welcome/sendwelcome.go
type Job struct {
	basejob.BaseJob
	UserID string `json:"user_id"`
}

func (*Job) Kind() string { return "mail.send_welcome" }

func (j *Job) Work(ctx context.Context) error {
	accounts, ok := accountservice.FromContext(ctx)
	if !ok {
		return fmt.Errorf("account service is missing from job context")
	}
	return accounts.SendWelcome(ctx, j.UserID)
}
```

```go
// app/jobs/jobs.go
func DefinedJobs(runner *lazyjobs.JobRunner) {
	runner.MustRegister(&sendwelcome.Job{})
}
```

```go
// init/app.go
func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name:         "sample_app",
		Drawer:       Draw,
		Public:       app.Public,
		Views:        app.Views,
		Dependencies: Dependencies,
		Jobs: lazyapp.Jobs(lazyjobs.Config{
			Define:  jobs.DefinedJobs,
			Workers: 2,
		}),
	})
}
```

Make payloads small and serializable, make service operations idempotent when
retries are possible, and return errors so the backend can record failure.

## Related

[Services](services.md) | [Mailers](mailers.md) | [App Anatomy](app-anatomy.md)
| [Testing](testing-verification.md)

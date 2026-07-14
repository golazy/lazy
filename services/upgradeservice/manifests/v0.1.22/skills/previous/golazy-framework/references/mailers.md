# Mailers

Mailers are communication adapters under `app/mailers`. They format and send
messages; services decide when a business event warrants a message and provide
the domain data.

## Initialize Delivery

Register delivery infrastructure in `init/dependencies.go`:

```go
_, err := lazydeps.Service(deps, "mailer", func(ctx context.Context) (
	context.Context,
	*lazymailer.Mailer,
	error,
	context.CancelFunc,
) {
	deliveries := lazymailer.NewRegistry("default", map[string]lazymailer.Delivery{
		"default": lazymailer.SMTPDelivery{
			Addr: os.Getenv("SMTP_ADDR"),
			Auth: smtp.PlainAuth(
				"",
				os.Getenv("SMTP_USER"),
				os.Getenv("SMTP_PASSWORD"),
				os.Getenv("SMTP_HOST"),
			),
		},
	})
	mailer, err := lazymailer.New(ctx, deliveries)
	if err != nil {
		return ctx, nil, fmt.Errorf("initialize mailer: %w", err), nil
	}
	return lazymailer.WithContext(ctx, mailer), mailer, nil, nil
})
```

## Define A Mailer

```go
// app/mailers/notice_mailer/noticemailer.go
type NoticeMailer struct {
	base lazymailer.Base
}

func New(ctx context.Context) (*NoticeMailer, error) {
	base, err := lazymailer.NewBase(ctx, "notice_mailer", lazymailer.Defaults{
		From:     lazymailer.MustParseAddress("GoLazy <hello@example.com>"),
		Delivery: "default",
		Layout:   "mailer",
	})
	if err != nil {
		return nil, err
	}
	return &NoticeMailer{base: base}, nil
}

func (m *NoticeMailer) Welcome(to lazymailer.Address, name string) error {
	m.base.Set("name", name)
	return m.base.Mail(lazymailer.Options{
		Action:  "welcome",
		To:      []lazymailer.Address{to},
		Subject: "Welcome",
	})
}
```

```gotemplate
{{/* app/views/notice_mailer/welcome.html.tpl */}}
<p>Hello {{.name}}</p>
```

Call the mailer from a controller or job adapter after a service returns the
business outcome, or inject a service-owned communication interface whose
implementation lives at the app boundary. Never import `app/mailers` from a
service. Keep recipient policy and event rules out of templates.

## Related

[Services](services.md) | [Jobs](jobs.md) | [Views](views.md)

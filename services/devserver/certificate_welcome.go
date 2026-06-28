package devserver

import (
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type certificateWelcomeData struct {
	Variant          instructionVariant
	Host             string
	HTTPSURL         string
	HTTPSProbeURL    string
	CertificateURL   string
	CertificateDir   string
	CertificatePath  string
	PrivateKeyPath   string
	InstructionLinks []instructionLink
	RequestedVariant string
}

type instructionVariant struct {
	Key     string
	Label   string
	Heading string
	Steps   []string
	Note    string
}

type instructionLink struct {
	Key      string
	Label    string
	URL      string
	Selected bool
}

func certificateWelcomePage(data certificateWelcomeData) []byte {
	var builder strings.Builder
	builder.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">")
	builder.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	builder.WriteString("<title>GoLazy Local HTTPS</title>")
	builder.WriteString("<style>")
	builder.WriteString(`:root{color-scheme:light;--blue:#00add8;--pink:#ce3262;--ink:#18222f;--muted:#5d6a78;--line:#d9e2ea;--soft:#f6f9fb;--warn:#7a4b00;--warn-bg:#fff7dc}*{box-sizing:border-box}body{margin:0;background:#fff;color:var(--ink);font:16px/1.5 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{min-height:100vh}.hero{background:linear-gradient(135deg,#f7fcfe 0%,#fff 52%,#fff7fa 100%);border-bottom:1px solid var(--line)}.inner{margin:0 auto;max-width:1060px;padding:32px 24px}.top{align-items:center;display:flex;gap:18px;justify-content:space-between}.logo{display:block;height:auto;width:min(230px,56vw)}.pill{border:1px solid var(--line);border-radius:999px;color:var(--muted);font-size:13px;padding:6px 12px;white-space:nowrap}.hero-grid{display:grid;gap:32px;grid-template-columns:minmax(0,1.2fr) minmax(300px,.8fr);padding:54px 0 28px}.eyebrow{color:var(--pink);font-size:13px;font-weight:750;letter-spacing:.08em;text-transform:uppercase}h1{font-size:clamp(2.1rem,5vw,4.3rem);line-height:.98;margin:.25rem 0 1rem;max-width:760px}p{margin:.75rem 0}.lead{color:#334155;font-size:1.12rem;max-width:680px}.actions{display:flex;flex-wrap:wrap;gap:12px;margin-top:24px}.button{align-items:center;border:1px solid var(--ink);border-radius:8px;color:var(--ink);display:inline-flex;font-weight:750;min-height:44px;padding:10px 14px;text-decoration:none}.button.primary{background:var(--ink);color:#fff}.button:hover{border-color:var(--pink);color:var(--pink)}.button.primary:hover{background:var(--pink);color:#fff}.status{background:#fff;border:1px solid var(--line);border-radius:8px;box-shadow:0 18px 60px rgba(24,34,47,.08);padding:20px}.status h2,.section h2{font-size:1.1rem;margin:0 0 .7rem}.host{align-items:center;background:var(--soft);border:1px solid var(--line);border-radius:8px;display:flex;gap:10px;margin-top:16px;padding:12px}.dot{background:var(--pink);border-radius:50%;height:10px;width:10px}.content{display:grid;gap:32px;grid-template-columns:minmax(0,.9fr) minmax(0,1.1fr);padding-bottom:56px;padding-top:36px}.section{min-width:0}.steps{counter-reset:step;display:grid;gap:12px;margin:18px 0 0}.step{background:#fff;border:1px solid var(--line);border-radius:8px;display:grid;gap:8px;grid-template-columns:34px minmax(0,1fr);padding:14px}.step::before{align-items:center;background:var(--blue);border-radius:50%;color:#fff;content:counter(step);counter-increment:step;display:flex;font-size:14px;font-weight:800;height:26px;justify-content:center;width:26px}.variants{display:flex;flex-wrap:wrap;gap:8px;margin:14px 0 22px}.variant{border:1px solid var(--line);border-radius:999px;color:var(--ink);font-size:14px;padding:7px 11px;text-decoration:none}.variant[aria-current=true]{background:var(--ink);border-color:var(--ink);color:#fff}.notice{background:var(--warn-bg);border:1px solid #f0d27a;border-radius:8px;color:var(--warn);padding:14px}.paths{display:grid;gap:10px;margin-top:14px}.path-row{background:var(--soft);border:1px solid var(--line);border-radius:8px;padding:11px}.path-row span{color:var(--muted);display:block;font-size:13px;margin-bottom:4px}code{font:13px/1.35 ui-monospace,SFMono-Regular,Consolas,monospace;overflow-wrap:anywhere}.small{color:var(--muted);font-size:14px}.probe{color:var(--muted);font-size:14px;margin-top:16px}@media (max-width:820px){.top{align-items:flex-start;flex-direction:column}.hero-grid,.content{grid-template-columns:1fr}.inner{padding-left:18px;padding-right:18px}.hero-grid{padding-top:36px}.pill{white-space:normal}}`)
	builder.WriteString("</style></head><body><main>")
	builder.WriteString("<section class=\"hero\"><div class=\"inner\">")
	builder.WriteString("<div class=\"top\"><img class=\"logo\" src=\"/_golazy/assets/golazy-horizontal.svg\" alt=\"GoLazy\"><div class=\"pill\">Local development HTTPS</div></div>")
	builder.WriteString("<div class=\"hero-grid\"><div>")
	builder.WriteString("<div class=\"eyebrow\">One-time trust setup</div>")
	builder.WriteString("<h1>Welcome to GoLazy local HTTPS</h1>")
	builder.WriteString("<p class=\"lead\">Install the GoLazy local development certificate authority once on this machine. After that, lazy can serve this app over HTTPS on the same port and the browser can reuse one secure connection for the development panel, reload stream, and app requests.</p>")
	builder.WriteString("<div class=\"actions\"><a class=\"button primary\" href=\"")
	builder.WriteString(html.EscapeString(data.CertificateURL))
	builder.WriteString("\">Download certificate authority</a><a class=\"button\" href=\"")
	builder.WriteString(html.EscapeString(data.HTTPSURL))
	builder.WriteString("\">Try HTTPS now</a></div>")
	builder.WriteString("<p class=\"probe\" id=\"https-probe-status\">Checking whether this browser already trusts the certificate authority...</p>")
	builder.WriteString("</div><aside class=\"status\">")
	builder.WriteString("<h2>This visit is for ")
	builder.WriteString(html.EscapeString(data.Host))
	builder.WriteString("</h2>")
	builder.WriteString("<p>Certificates are created on demand for each domain. If you switch between <code>localhost</code>, <code>127.0.0.1</code>, or a custom name such as <code>dev.local</code>, lazy will create a matching HTTPS certificate for that host.</p>")
	builder.WriteString("<div class=\"host\"><span class=\"dot\"></span><span>Trust the CA once; use HTTPS for each local domain after that.</span></div>")
	builder.WriteString("</aside></div></div></section>")
	builder.WriteString("<section class=\"inner content\"><div class=\"section\">")
	builder.WriteString("<h2>Choose instructions</h2><p class=\"small\">The page tries to detect your operating system. You can switch instructions manually.</p><nav class=\"variants\" aria-label=\"Instruction variants\">")
	for _, link := range data.InstructionLinks {
		builder.WriteString("<a class=\"variant\" href=\"")
		builder.WriteString(html.EscapeString(link.URL))
		builder.WriteString("\"")
		if link.Selected {
			builder.WriteString(" aria-current=\"true\"")
		}
		builder.WriteString(">")
		builder.WriteString(html.EscapeString(link.Label))
		builder.WriteString("</a>")
	}
	builder.WriteString("</nav><div class=\"notice\"><strong>Do not share these files.</strong> The private key lets lazy mint trusted local certificates. Keep the data directory private and never upload it, commit it, or send it to someone else.</div>")
	builder.WriteString("<div class=\"paths\"><div class=\"path-row\"><span>Data directory</span><code>")
	builder.WriteString(html.EscapeString(data.CertificateDir))
	builder.WriteString("</code></div><div class=\"path-row\"><span>Public certificate authority</span><code>")
	builder.WriteString(html.EscapeString(data.CertificatePath))
	builder.WriteString("</code></div><div class=\"path-row\"><span>Private key</span><code>")
	builder.WriteString(html.EscapeString(data.PrivateKeyPath))
	builder.WriteString("</code></div></div>")
	builder.WriteString("<p class=\"small\">GoLazy does not install the certificate automatically because trust stores are operating-system and browser specific, and some flows require administrator approval.</p>")
	builder.WriteString("</div><div class=\"section\"><h2>")
	builder.WriteString(html.EscapeString(data.Variant.Heading))
	builder.WriteString("</h2><div class=\"steps\">")
	for _, step := range data.Variant.Steps {
		builder.WriteString("<div class=\"step\"><div>")
		builder.WriteString(step)
		builder.WriteString("</div></div>")
	}
	builder.WriteString("</div>")
	if data.Variant.Note != "" {
		builder.WriteString("<p class=\"small\">")
		builder.WriteString(data.Variant.Note)
		builder.WriteString("</p>")
	}
	builder.WriteString("</div></section>")
	builder.WriteString("<script>")
	builder.WriteString("const requestedVariant=")
	builder.WriteString(strconv.Quote(data.RequestedVariant))
	builder.WriteString(";const probeURL=")
	builder.WriteString(strconv.Quote(data.HTTPSProbeURL))
	builder.WriteString(";const redirectURL=")
	builder.WriteString(strconv.Quote(data.HTTPSURL))
	builder.WriteString(`;function platformVariant(){const platform=(navigator.userAgentData&&navigator.userAgentData.platform||navigator.platform||navigator.userAgent||"").toLowerCase();if(platform.includes("mac"))return"macos";if(platform.includes("win"))return"windows";if(platform.includes("linux")||platform.includes("x11"))return"linux";return""}if(!requestedVariant){const variant=platformVariant();if(variant){const next=new URL(window.location.href);next.searchParams.set("os",variant);window.location.replace(next.toString())}}fetch(probeURL,{mode:"no-cors",cache:"no-store"}).then(()=>{window.location.replace(redirectURL)}).catch(()=>{const target=document.getElementById("https-probe-status");if(target)target.textContent="HTTPS is waiting for this browser to trust the GoLazy certificate authority."});`)
	builder.WriteString("</script></main></body></html>")
	return []byte(builder.String())
}

func instructionVariantForRequest(r *http.Request) instructionVariant {
	key := normalizeInstructionKey(r.URL.Query().Get("os"))
	if key == "" {
		key = detectInstructionKey(r.UserAgent())
	}
	if key == "" {
		key = "generic"
	}
	for _, variant := range instructionVariants() {
		if variant.Key == key {
			return variant
		}
	}
	return genericInstructionVariant()
}

func instructionLinksForRequest(r *http.Request) []instructionLink {
	selected := instructionVariantForRequest(r).Key
	var links []instructionLink
	for _, variant := range instructionVariants() {
		next := *r.URL
		if next.Path == "" {
			next.Path = "/"
		}
		query := next.Query()
		query.Set("os", variant.Key)
		next.RawQuery = query.Encode()
		links = append(links, instructionLink{
			Key:      variant.Key,
			Label:    variant.Label,
			URL:      next.RequestURI(),
			Selected: variant.Key == selected,
		})
	}
	return links
}

func detectInstructionKey(userAgent string) string {
	userAgent = strings.ToLower(userAgent)
	switch {
	case strings.Contains(userAgent, "mac os x"), strings.Contains(userAgent, "macintosh"):
		return "macos"
	case strings.Contains(userAgent, "windows"):
		return "windows"
	case strings.Contains(userAgent, "linux"), strings.Contains(userAgent, "x11"):
		return "linux"
	default:
		return ""
	}
}

func normalizeInstructionKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mac", "macos", "darwin":
		return "macos"
	case "win", "windows":
		return "windows"
	case "linux":
		return "linux"
	case "generic", "other":
		return "generic"
	default:
		return ""
	}
}

func instructionVariants() []instructionVariant {
	return []instructionVariant{
		{
			Key:     "macos",
			Label:   "macOS",
			Heading: "Install on macOS",
			Steps: []string{
				`Download the certificate authority file from this page.`,
				`Open <strong>Keychain Access</strong>, select the <strong>System</strong> keychain when you can approve administrator prompts, or the <strong>login</strong> keychain for the current user only.`,
				`Choose <strong>File > Import Items</strong>, select the downloaded GoLazy certificate authority, then open the imported certificate.`,
				`Expand <strong>Trust</strong>, set <strong>Secure Sockets Layer (SSL)</strong> to <strong>Always Trust</strong>, close the window, and approve the change.`,
				`Reload the HTTP page. GoLazy will detect that HTTPS works and move you to the secure local URL.`,
			},
		},
		{
			Key:     "windows",
			Label:   "Windows",
			Heading: "Install on Windows",
			Steps: []string{
				`Download the certificate authority file from this page.`,
				`Open <strong>Manage user certificates</strong> from the Start menu, or run <code>certmgr.msc</code>.`,
				`Open <strong>Trusted Root Certification Authorities</strong>, right-click <strong>Certificates</strong>, then choose <strong>All Tasks > Import</strong>.`,
				`Import the downloaded GoLazy certificate authority into the current user's trusted root store.`,
				`Restart your browser or reload the HTTP page. GoLazy will redirect to HTTPS once the trust store is active.`,
			},
		},
		{
			Key:     "linux",
			Label:   "Linux",
			Heading: "Install on Linux",
			Steps: []string{
				`Download the certificate authority file from this page.`,
				`For Chrome or Chromium, install the NSS tools for your distribution, then add the certificate to the user NSS database with <code>certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "GoLazy Local Development CA" -i ~/Downloads/golazy-local-development-ca.pem</code>.`,
				`For Firefox, open <strong>Settings > Privacy &amp; Security > Certificates > View Certificates > Authorities</strong>, then import the downloaded file and trust it for websites.`,
				`Restart the browser or reload the HTTP page. GoLazy will redirect to HTTPS when the browser trusts the local CA.`,
			},
			Note: `Linux trust stores vary by distribution and browser. Prefer a user or browser trust store for local development unless you intentionally want a system-wide root.`,
		},
		genericInstructionVariant(),
	}
}

func genericInstructionVariant() instructionVariant {
	return instructionVariant{
		Key:     "generic",
		Label:   "Generic",
		Heading: "Install manually",
		Steps: []string{
			`Download the GoLazy certificate authority file from this page.`,
			`Open your operating system or browser certificate manager.`,
			`Import the downloaded file as a trusted root certificate authority for website or SSL certificates.`,
			`Restart the browser if needed, then reload this HTTP page. GoLazy will redirect to HTTPS after the trust change is visible.`,
		},
		Note: `The CA is for local development on this machine. Do not share the data directory or private key.`,
	}
}

func httpsURLForRequest(r *http.Request) string {
	next := *r.URL
	if next.Path == "" {
		next.Path = "/"
	}
	next.Scheme = "https"
	next.Host = r.Host
	return next.String()
}

func httpsProbeURLForRequest(r *http.Request) string {
	probe := url.URL{
		Scheme: "https",
		Host:   r.Host,
		Path:   HTTPSProbePath,
	}
	return probe.String()
}

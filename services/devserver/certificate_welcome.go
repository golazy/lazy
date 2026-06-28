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
	Key      string
	Label    string
	Heading  string
	Steps    []string
	Commands []copyCommand
	Note     string
}

type copyCommand struct {
	Label string
	Value string
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
	builder.WriteString(`:root{color-scheme:light;--blue:#00add8;--pink:#ce3262;--ink:#18222f;--muted:#5d6876;--line:#dbe4ec;--soft:#f6f9fb;--warn:#7a4b00;--warn-bg:#fff7dc}*{box-sizing:border-box}body{margin:0;background:#fff;color:var(--ink);font:16px/1.45 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}.wrap{margin:0 auto;max-width:1040px;padding:28px 24px}.top{align-items:center;border-bottom:1px solid var(--line);display:flex;gap:18px;justify-content:space-between;padding-bottom:22px}.logo{display:block;height:auto;width:min(220px,58vw)}.badge{border:1px solid var(--line);border-radius:999px;color:var(--muted);font-size:13px;padding:6px 12px;white-space:nowrap}.hero{display:grid;gap:34px;grid-template-columns:minmax(0,1fr) minmax(320px,.78fr);padding:46px 0 30px}.eyebrow{color:var(--pink);font-size:13px;font-weight:800;letter-spacing:.08em;text-transform:uppercase}h1{font-size:clamp(2.15rem,5vw,4.35rem);line-height:1;margin:.3rem 0 .9rem;max-width:720px}h2{font-size:1.14rem;margin:0 0 12px}p{margin:.65rem 0}.lead{color:#344255;font-size:1.1rem;max-width:620px}.actions{display:flex;flex-wrap:wrap;gap:12px;margin-top:22px}.button{align-items:center;border:1px solid var(--ink);border-radius:8px;color:var(--ink);display:inline-flex;font-weight:750;min-height:44px;padding:10px 14px;text-decoration:none}.button.primary{background:var(--ink);color:#fff}.button:hover{border-color:var(--pink);color:var(--pink)}.button.primary:hover{background:var(--pink);color:#fff}.probe,.small{color:var(--muted);font-size:14px}.panel{background:#fff;border:1px solid var(--line);border-radius:8px;box-shadow:0 18px 56px rgba(24,34,47,.08);padding:20px}.meta{display:grid;gap:10px;margin-top:16px}.meta-row{background:var(--soft);border:1px solid var(--line);border-radius:8px;padding:11px}.meta-row span,summary{color:var(--muted);font-size:13px}code{font:13px/1.35 ui-monospace,SFMono-Regular,Consolas,monospace;overflow-wrap:anywhere}.content{display:grid;gap:34px;grid-template-columns:minmax(0,.42fr) minmax(0,.58fr);padding-bottom:48px}.variants{display:flex;flex-wrap:wrap;gap:8px;margin:12px 0 18px}.variant{border:1px solid var(--line);border-radius:999px;color:var(--ink);font-size:14px;padding:7px 11px;text-decoration:none}.variant[aria-current=true]{background:var(--ink);border-color:var(--ink);color:#fff}.notice{background:var(--warn-bg);border:1px solid #f0d27a;border-radius:8px;color:var(--warn);padding:13px}.steps{counter-reset:step;display:grid;gap:10px;margin:16px 0}.step{align-items:start;background:#fff;border:1px solid var(--line);border-radius:8px;display:grid;gap:11px;grid-template-columns:28px minmax(0,1fr);padding:13px}.step:before{align-items:center;background:var(--blue);border-radius:50%;color:#fff;content:counter(step);counter-increment:step;display:flex;font-size:13px;font-weight:800;height:24px;justify-content:center;width:24px}.commands{display:grid;gap:10px;margin:14px 0}.command{align-items:center;background:var(--soft);border:1px solid var(--line);border-radius:8px;display:grid;gap:10px;grid-template-columns:minmax(0,1fr) auto;padding:12px}.command span{color:var(--muted);display:block;font-size:13px;margin-bottom:4px}.copy-button{background:#fff;border:1px solid var(--ink);border-radius:8px;color:var(--ink);cursor:pointer;font:inherit;font-size:14px;font-weight:750;min-height:38px;padding:8px 12px}.copy-button:hover{border-color:var(--pink);color:var(--pink)}details{border-top:1px solid var(--line);margin-top:16px;padding-top:12px}summary{cursor:pointer;font-weight:700}.paths{display:grid;gap:8px;margin-top:10px}.path-row{background:var(--soft);border:1px solid var(--line);border-radius:8px;padding:10px}@media (max-width:820px){.wrap{padding-left:18px;padding-right:18px}.top{align-items:flex-start;flex-direction:column}.badge{white-space:normal}.hero,.content{grid-template-columns:1fr}.hero{padding-top:34px}.command{grid-template-columns:1fr}.copy-button{justify-content:center;width:100%}}`)
	builder.WriteString("</style></head><body><main><div class=\"wrap\">")
	builder.WriteString("<header class=\"top\"><img class=\"logo\" src=\"/_golazy/assets/golazy-horizontal.svg\" alt=\"GoLazy\"><div class=\"badge\">Local development HTTPS</div></header>")
	builder.WriteString("<section class=\"hero\"><div>")
	builder.WriteString("<div class=\"eyebrow\">One-time trust setup</div>")
	builder.WriteString("<h1>Welcome to GoLazy local HTTPS</h1>")
	builder.WriteString("<p class=\"lead\">Install the local GoLazy certificate authority once on this machine. Then this app and the development panel can use HTTPS on the same port.</p>")
	builder.WriteString("<div class=\"actions\"><a class=\"button primary\" href=\"")
	builder.WriteString(html.EscapeString(data.CertificateURL))
	builder.WriteString("\">Download certificate authority</a><a class=\"button\" href=\"")
	builder.WriteString(html.EscapeString(data.HTTPSURL))
	builder.WriteString("\">Try HTTPS now</a></div>")
	builder.WriteString("<p class=\"probe\" id=\"https-probe-status\">Checking HTTPS trust...</p>")
	builder.WriteString("</div><aside class=\"panel\"><h2>")
	builder.WriteString(html.EscapeString(data.Host))
	builder.WriteString("</h2><p>Certificates are created per local domain. Switching between <code>localhost</code>, <code>127.0.0.1</code>, and custom names starts a new host certificate.</p>")
	builder.WriteString("<div class=\"meta\"><div class=\"meta-row\"><span>Secure URL</span><br><code>")
	builder.WriteString(html.EscapeString(data.HTTPSURL))
	builder.WriteString("</code></div><div class=\"meta-row\"><span>Setup</span><br>Usually once per machine or browser profile.</div></div>")
	builder.WriteString("</aside></section>")
	builder.WriteString("<section class=\"content\"><div>")
	builder.WriteString("<h2>Instructions</h2><p class=\"small\">Detected automatically. Switch if needed.</p><nav class=\"variants\" aria-label=\"Instruction variants\">")
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
	builder.WriteString("</nav><div class=\"notice\"><strong>Do not share these files.</strong> The private key can create trusted local certificates.</div>")
	builder.WriteString("<details><summary>Certificate files</summary><div class=\"paths\"><div class=\"path-row\"><span>Data directory</span><br><code>")
	builder.WriteString(html.EscapeString(data.CertificateDir))
	builder.WriteString("</code></div><div class=\"path-row\"><span>Public certificate authority</span><br><code>")
	builder.WriteString(html.EscapeString(data.CertificatePath))
	builder.WriteString("</code></div><div class=\"path-row\"><span>Private key</span><br><code>")
	builder.WriteString(html.EscapeString(data.PrivateKeyPath))
	builder.WriteString("</code></div></div></details>")
	builder.WriteString("<p class=\"small\">Automatic install is not offered because trust stores and approval prompts vary by OS and browser.</p>")
	builder.WriteString("</div><div><h2>")
	builder.WriteString(html.EscapeString(data.Variant.Heading))
	builder.WriteString("</h2><div class=\"steps\">")
	for _, step := range data.Variant.Steps {
		builder.WriteString("<div class=\"step\"><div>")
		builder.WriteString(step)
		builder.WriteString("</div></div>")
	}
	builder.WriteString("</div>")
	if len(data.Variant.Commands) > 0 {
		builder.WriteString("<div class=\"commands\" aria-label=\"Commands\">")
		for _, command := range data.Variant.Commands {
			builder.WriteString("<div class=\"command\"><div><span>")
			builder.WriteString(html.EscapeString(command.Label))
			builder.WriteString("</span><code>")
			builder.WriteString(html.EscapeString(command.Value))
			builder.WriteString("</code></div><button type=\"button\" class=\"copy-button\" data-copy=\"")
			builder.WriteString(html.EscapeString(command.Value))
			builder.WriteString("\">Copy</button></div>")
		}
		builder.WriteString("</div>")
	}
	if data.Variant.Note != "" {
		builder.WriteString("<p class=\"small\">")
		builder.WriteString(data.Variant.Note)
		builder.WriteString("</p>")
	}
	builder.WriteString("</div></section></div>")
	builder.WriteString("<script>")
	builder.WriteString("const requestedVariant=")
	builder.WriteString(strconv.Quote(data.RequestedVariant))
	builder.WriteString(";const probeURL=")
	builder.WriteString(strconv.Quote(data.HTTPSProbeURL))
	builder.WriteString(";const redirectURL=")
	builder.WriteString(strconv.Quote(data.HTTPSURL))
	builder.WriteString(`;function platformVariant(){const platform=(navigator.userAgentData&&navigator.userAgentData.platform||navigator.platform||navigator.userAgent||"").toLowerCase();if(platform.includes("mac"))return"macos";if(platform.includes("win"))return"windows";if(platform.includes("linux")||platform.includes("x11"))return"linux";return""}async function copyText(text){if(navigator.clipboard&&window.isSecureContext){await navigator.clipboard.writeText(text);return}const area=document.createElement("textarea");area.value=text;area.setAttribute("readonly","");area.style.position="fixed";area.style.left="-9999px";document.body.appendChild(area);area.select();document.execCommand("copy");area.remove()}document.addEventListener("click",async(event)=>{const button=event.target.closest&&event.target.closest("[data-copy]");if(!button)return;const original=button.textContent;try{await copyText(button.dataset.copy);button.textContent="Copied";setTimeout(()=>{button.textContent=original},1500)}catch(_){button.textContent="Copy failed";setTimeout(()=>{button.textContent=original},1500)}});if(!requestedVariant){const variant=platformVariant();if(variant){const next=new URL(window.location.href);next.searchParams.set("os",variant);window.location.replace(next.toString())}}fetch(probeURL,{mode:"no-cors",cache:"no-store"}).then(()=>{window.location.replace(redirectURL)}).catch(()=>{const target=document.getElementById("https-probe-status");if(target)target.textContent="HTTPS is waiting for local trust."});`)
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
				`Download the certificate authority.`,
				`Import it in <strong>Keychain Access</strong>.`,
				`Set <strong>Secure Sockets Layer (SSL)</strong> to <strong>Always Trust</strong>.`,
				`Reload this page.`,
			},
		},
		{
			Key:     "windows",
			Label:   "Windows",
			Heading: "Install on Windows",
			Steps: []string{
				`Download the certificate authority.`,
				`Open <strong>Manage user certificates</strong>.`,
				`Import it into <strong>Trusted Root Certification Authorities</strong>.`,
				`Reload this page.`,
			},
			Commands: []copyCommand{
				{
					Label: "Open the certificate manager",
					Value: "certmgr.msc",
				},
			},
		},
		{
			Key:     "linux",
			Label:   "Linux",
			Heading: "Install on Linux",
			Steps: []string{
				`Download the certificate authority.`,
				`Import it into your browser or user trust store.`,
				`Restart the browser if needed.`,
				`Reload this page.`,
			},
			Commands: []copyCommand{
				{
					Label: "Chrome or Chromium user trust store",
					Value: `certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "GoLazy Local Development CA" -i ~/Downloads/golazy-local-development-ca.pem`,
				},
			},
			Note: `Firefox can also import the CA from Settings > Privacy &amp; Security > Certificates.`,
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
			`Download the certificate authority.`,
			`Open your OS or browser certificate manager.`,
			`Import it as a trusted root certificate authority for websites.`,
			`Reload this page.`,
		},
		Note: `Use a browser or user trust store unless you intentionally want system-wide trust.`,
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

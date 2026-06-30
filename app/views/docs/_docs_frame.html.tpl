<section id="docs" class="tool-view is-active docs-view" data-view="docs">
  <div class="filter-toolbar">
    <form method="get" action="{{.docs.BaseURL}}" class="inline-form">
      {{if .docs.SelectedModule}}<input type="hidden" name="module" value="{{.docs.SelectedModule}}">{{end}}
      {{if .docs.SelectedSlug}}<input type="hidden" name="pkg" value="{{.docs.SelectedSlug}}">{{end}}
      <input class="filter-input" type="search" name="q" placeholder="Search project docs" value="{{.docs.Query}}">
    </form>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count">{{.docs.CountText}}</span>
  </div>

  <div class="docs-layout">
    <aside class="docs-sidebar" aria-label="Project packages">
      <div class="docs-module">
        {{if .docs.Module}}<code>{{.docs.Module}}</code>{{else}}Project docs{{end}}
      </div>
      {{if .docs.Modules}}
        <div class="docs-sidebar-heading">Modules</div>
        <nav class="docs-module-list">
          {{range .docs.Modules}}
            <a class="{{if .Current}}is-current{{end}}" href="{{.URL}}" data-turbo-frame="_top">
              <span>{{.Label}}</span>
              <code>{{.Path}}</code>
              {{with .Version}}<small>{{.}}</small>{{end}}
            </a>
          {{end}}
        </nav>
        {{with .docs.ModulesError}}<p class="docs-sidebar-note">{{.}}</p>{{end}}
        <div class="docs-sidebar-heading">Packages</div>
      {{end}}
      <nav class="docs-package-list">
        {{range .docs.Packages}}
          <a class="{{if .Current}}is-current{{end}}" href="{{.URL}}" data-turbo-frame="_top">
            <span>{{.Label}}</span>
            {{with .Synopsis}}<small>{{.}}</small>{{end}}
          </a>
        {{end}}
      </nav>
    </aside>

    <section class="docs-content" aria-label="Package documentation">
      {{if .docs.Error}}
        <div class="empty-state">
          <h2>Docs unavailable</h2>
          <p>{{.docs.Error}}</p>
        </div>
      {{else}}
        {{if .docs.HasSearch}}
          <section class="docs-search-results" aria-label="Search results">
            <h2>Search</h2>
            {{if .docs.Search}}
              <ol>
                {{range .docs.Search}}
                  <li>
                    <a href="{{.URL}}" data-turbo-frame="_top"><span>{{.Kind}}</span> {{.Name}}</a>
                    <small>{{.Path}}</small>
                    {{with .Summary}}<p>{{.}}</p>{{end}}
                  </li>
                {{end}}
              </ol>
            {{else}}
              <p class="muted">No matches for <code>{{.docs.Query}}</code>.</p>
            {{end}}
          </section>
        {{end}}

        {{with .docs.SourceError}}<p class="muted">{{.}}</p>{{end}}
        {{with .docs.Source}}
          <section class="docs-source-view" aria-label="Source file">
            <div class="docs-source-toolbar">
              <code>{{.Path}}</code>
              <span>line {{.Line}}</span>
            </div>
            <pre>{{range .Lines}}<code class="{{if .Highlighted}}is-highlighted{{end}}"><span>{{.Number}}</span>{{.Text}}</code>
{{end}}</pre>
          </section>
        {{end}}

        {{with .docs.Selected}}
          {{$pkg := .}}
          <article class="docs-package">
            <header>
              <h1>package {{.Name}}</h1>
              <pre><code>import "{{.ImportPath}}"</code></pre>
              {{with $.docs.SourceURL .Source}}<a class="source-link" href="{{.}}" data-turbo-frame="_top">Source</a>{{end}}
              {{with .Doc}}<p class="doc-comment">{{.}}</p>{{end}}
            </header>

            <nav class="docs-symbol-index" aria-label="Symbol index">
              {{if .Constants}}<a href="#constants">Constants</a>{{end}}
              {{if .Variables}}<a href="#variables">Variables</a>{{end}}
              {{if .Functions}}<a href="#functions">Functions</a>{{end}}
              {{if .Types}}<a href="#types">Types</a>{{end}}
              {{if .Examples}}<a href="#examples">Examples</a>{{end}}
            </nav>

            {{if .Constants}}
              <section id="constants" class="docs-section">
                <h2>Constants</h2>
                {{range .Constants}}
                  {{$symbol := index .Names 0}}
                  <article id="{{$symbol}}" class="docs-symbol">
                    <h3><a href="{{$.docs.SourceURL .Source}}" data-turbo-frame="_top">const {{$symbol}}</a></h3>
                    {{with .Doc}}<p class="doc-comment">{{.}}</p>{{end}}
                    <pre><code>{{.Decl}}</code></pre>
                  </article>
                {{end}}
              </section>
            {{end}}

            {{if .Variables}}
              <section id="variables" class="docs-section">
                <h2>Variables</h2>
                {{range .Variables}}
                  {{$symbol := index .Names 0}}
                  <article id="{{$symbol}}" class="docs-symbol">
                    <h3><a href="{{$.docs.SourceURL .Source}}" data-turbo-frame="_top">var {{$symbol}}</a></h3>
                    {{with .Doc}}<p class="doc-comment">{{.}}</p>{{end}}
                    <pre><code>{{.Decl}}</code></pre>
                  </article>
                {{end}}
              </section>
            {{end}}

            {{if .Functions}}
              <section id="functions" class="docs-section">
                <h2>Functions</h2>
                {{range .Functions}}
                  <article id="{{.Name}}" class="docs-symbol">
                    <h3><a href="{{$.docs.SourceURL .Source}}" data-turbo-frame="_top">func {{.Name}}</a></h3>
                    {{with .Doc}}<p class="doc-comment">{{.}}</p>{{end}}
                    <pre><code>{{.Decl}}</code></pre>
                    {{template "docs_examples" .Examples}}
                  </article>
                {{end}}
              </section>
            {{end}}

            {{if .Types}}
              <section id="types" class="docs-section">
                <h2>Types</h2>
                {{range .Types}}
                  {{$type := .}}
                  <article id="{{.Name}}" class="docs-symbol">
                    <h3><a href="{{$.docs.SourceURL .Source}}" data-turbo-frame="_top">type {{.Name}}</a></h3>
                    {{with .Doc}}<p class="doc-comment">{{.}}</p>{{end}}
                    <pre><code>{{.Decl}}</code></pre>
                    {{template "docs_examples" .Examples}}

                    {{range .Funcs}}
                      <section id="{{.Name}}" class="docs-method">
                        <h4><a href="{{$.docs.SourceURL .Source}}" data-turbo-frame="_top">func {{.Name}}</a></h4>
                        {{with .Doc}}<p class="doc-comment">{{.}}</p>{{end}}
                        <pre><code>{{.Decl}}</code></pre>
                        {{template "docs_examples" .Examples}}
                      </section>
                    {{end}}

                    {{range .Methods}}
                      {{$methodSymbol := printf "%s.%s" $type.Name .Name}}
                      <section id="{{$methodSymbol}}" class="docs-method">
                        <h4><a href="{{$.docs.SourceURL .Source}}" data-turbo-frame="_top">func {{.Name}}</a></h4>
                        {{with .Doc}}<p class="doc-comment">{{.}}</p>{{end}}
                        <pre><code>{{.Decl}}</code></pre>
                        {{template "docs_examples" .Examples}}
                      </section>
                    {{end}}
                  </article>
                {{end}}
              </section>
            {{end}}

            {{if .Examples}}
              <section id="examples" class="docs-section">
                <h2>Examples</h2>
                {{template "docs_examples" .Examples}}
              </section>
            {{end}}
          </article>
        {{end}}
      {{end}}
    </section>
  </div>
</section>

{{define "docs_examples"}}
  {{if .}}
    <div class="docs-examples">
      {{range .}}
        <figure>
          {{with .Doc}}<figcaption>{{.}}</figcaption>{{end}}
          <pre><code>{{.Code}}</code></pre>
          {{with .Output}}<pre class="doc-output"><code>{{.}}</code></pre>{{end}}
        </figure>
      {{end}}
    </div>
  {{end}}
{{end}}

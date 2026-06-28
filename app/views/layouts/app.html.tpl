<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>GoLazy Development Panel</title>
    {{stylesheet "/assets/devtools.css"}}
    {{stylesheet "/assets/panel.css"}}
    {{importmap "/assets/importmap.json"}}
    <script type="module">import "/js/app.js"</script>
  </head>
  <body>
    <main class="devtools-panel" data-panel data-state="{{.state.State}}">
      <div class="panel-main">
        {{.content}}
      </div>
      <turbo-frame id="status_bar" src="{{path_for "status"}}" data-turbo-permanent></turbo-frame>
    </main>
  </body>
</html>

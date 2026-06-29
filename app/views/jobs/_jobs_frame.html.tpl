<section id="jobs" class="tool-view is-active" data-view="jobs">
    <div class="filter-toolbar">
    <input class="filter-input" type="search" placeholder="Filter jobs" disabled>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count">{{.jobs.StateText}}</span>
  </div>

  <div class="runtime-grid" data-jobs-panel>
    <section class="runtime-pane">
      <h2>State</h2>
      <dl class="detail-list">
        <dt>Runner</dt>
        <dd>{{.jobs.RunningText}}</dd>
        <dt>Total</dt>
        <dd>{{.jobs.Stats.Total}}</dd>
        <dt>Pending</dt>
        <dd>{{.jobs.Count "pending"}}</dd>
        <dt>Running</dt>
        <dd>{{.jobs.Count "running"}}</dd>
        <dt>Retrying</dt>
        <dd>{{.jobs.Count "retrying"}}</dd>
        <dt>Succeeded</dt>
        <dd>{{.jobs.Count "succeeded"}}</dd>
        <dt>Discarded</dt>
        <dd>{{.jobs.Count "discarded"}}</dd>
      </dl>
    </section>

    <section class="runtime-pane output-pane">
      <h2>Definitions</h2>
      <ul class="compact-list" data-job-definitions>
        {{if .defer_panel_lists}}
        {{else}}
          {{partial "job_definitions" .}}
        {{end}}
      </ul>
    </section>

    <section class="runtime-pane output-pane">
      <h2>Recent Jobs</h2>
      <table class="data-grid">
        <thead>
          <tr>
            <th>ID</th>
            <th>Kind</th>
            <th>Queue</th>
            <th>State</th>
            <th>Attempt</th>
            <th>Run At</th>
            <th>Error</th>
          </tr>
        </thead>
        <tbody data-jobs-recent>
          {{if .defer_panel_lists}}
          {{else}}
            {{partial "recent_jobs" .}}
          {{end}}
        </tbody>
      </table>
    </section>
  </div>
</section>

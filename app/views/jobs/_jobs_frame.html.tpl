<section class="tool-view" data-view="jobs">
  <div class="filter-toolbar">
    <input class="filter-input" type="search" placeholder="Filter jobs" disabled>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-jobs-state>Jobs unavailable</span>
  </div>

  <div class="runtime-grid" data-jobs-panel>
    <section class="runtime-pane">
      <h2>State</h2>
      <dl class="detail-list">
        <dt>Runner</dt>
        <dd data-jobs-running>Unknown</dd>
        <dt>Total</dt>
        <dd data-jobs-total>0</dd>
        <dt>Pending</dt>
        <dd data-jobs-pending>0</dd>
        <dt>Running</dt>
        <dd data-jobs-count-running>0</dd>
        <dt>Retrying</dt>
        <dd data-jobs-retrying>0</dd>
        <dt>Succeeded</dt>
        <dd data-jobs-succeeded>0</dd>
        <dt>Discarded</dt>
        <dd data-jobs-discarded>0</dd>
      </dl>
    </section>

    <section class="runtime-pane output-pane">
      <h2>Definitions</h2>
      <ul class="compact-list" data-job-definitions>
        <li class="muted">No job definitions.</li>
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
          <tr>
            <td colspan="7" class="empty-cell">No recent jobs.</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</section>

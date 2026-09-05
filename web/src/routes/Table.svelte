<script lang="ts">
  // Placeholder: Task 20 replaces this with the table/match spectator view.
  let { table, match }: { table: string; match?: number } = $props();
</script>

<main>
  <h1>Table {table}{match === undefined ? '' : ` / match ${match}`}</h1>
</main>

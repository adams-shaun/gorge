<script lang="ts">
  /**
   * A persistent feedback affordance, mounted once at the app root so it
   * appears above every route. Capturing a screenshot uses the browser's own
   * screen-sharing prompt (getDisplayMedia) rather than a DOM-rasterizing
   * library — one grant, one frame, no dependency. It is optional: a report
   * with just text and no screenshot is a normal, complete submission.
   */
  let open = $state(false);
  let text = $state('');
  let screenshot = $state<Blob | null>(null);
  let screenshotURL = $state<string | null>(null);
  let capturing = $state(false);
  let submitting = $state(false);
  let result = $state<'ok' | 'error' | null>(null);

  function reset(): void {
    text = '';
    screenshot = null;
    if (screenshotURL) URL.revokeObjectURL(screenshotURL);
    screenshotURL = null;
    result = null;
  }

  function toggle(): void {
    open = !open;
    if (!open) reset();
  }

  async function captureScreen(): Promise<void> {
    capturing = true;
    try {
      const stream = await navigator.mediaDevices.getDisplayMedia({ video: true });
      const track = stream.getVideoTracks()[0];
      const bitmap = await new ImageCapture(track).grabFrame();
      track.stop();
      const canvas = document.createElement('canvas');
      canvas.width = bitmap.width;
      canvas.height = bitmap.height;
      canvas.getContext('2d')?.drawImage(bitmap, 0, 0);
      const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
      if (blob) {
        if (screenshotURL) URL.revokeObjectURL(screenshotURL);
        screenshot = blob;
        screenshotURL = URL.createObjectURL(blob);
      }
    } catch {
      // Permission denied or cancelled — leave any existing screenshot as is.
    } finally {
      capturing = false;
    }
  }

  function clearScreenshot(): void {
    if (screenshotURL) URL.revokeObjectURL(screenshotURL);
    screenshot = null;
    screenshotURL = null;
  }

  async function submit(): Promise<void> {
    if (!text.trim()) return;
    submitting = true;
    result = null;
    try {
      const form = new FormData();
      form.set('text', text);
      form.set('url', location.href);
      if (screenshot) form.set('screenshot', screenshot, 'screenshot.png');
      const res = await fetch('/api/feedback', { method: 'POST', body: form });
      result = res.ok ? 'ok' : 'error';
      if (res.ok) {
        text = '';
        clearScreenshot();
      }
    } catch {
      result = 'error';
    } finally {
      submitting = false;
    }
  }
</script>

<button type="button" class="feedback-badge" onclick={toggle} aria-haspopup="dialog" aria-expanded={open}>
  Feedback
</button>

{#if open}
  <div class="feedback-panel" role="dialog" aria-label="Send feedback">
    <textarea
      class="feedback-text"
      placeholder="What happened?"
      bind:value={text}
      rows="4"
    ></textarea>
    <div class="feedback-shot">
      {#if screenshotURL}
        <img src={screenshotURL} alt="Captured screenshot" class="feedback-thumb" />
        <button type="button" class="feedback-link" onclick={clearScreenshot}>Remove screenshot</button>
      {:else}
        <button type="button" class="feedback-link" onclick={captureScreen} disabled={capturing}>
          {capturing ? 'Choose what to share…' : 'Attach a screenshot'}
        </button>
      {/if}
    </div>
    <div class="feedback-actions">
      {#if result === 'ok'}<span class="feedback-status feedback-status--ok">Sent — thank you.</span>{/if}
      {#if result === 'error'}<span class="feedback-status feedback-status--error">Couldn't send. Try again.</span>{/if}
      <button type="button" class="feedback-cancel" onclick={toggle}>Cancel</button>
      <button type="button" class="feedback-submit" onclick={submit} disabled={submitting || !text.trim()}>
        {submitting ? 'Sending…' : 'Send'}
      </button>
    </div>
  </div>
{/if}

<style>
  .feedback-badge {
    position: fixed;
    top: var(--sp-2);
    right: var(--sp-2);
    z-index: 30;
    padding: var(--sp-1) var(--sp-3);
    border: var(--edge-w, 1px) solid var(--edge-inst);
    border-radius: var(--radius);
    background: var(--instrument);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    font-weight: 600;
    cursor: pointer;
  }
  .feedback-badge:hover,
  .feedback-badge[aria-expanded='true'] {
    border-color: var(--ink-dim);
    color: var(--ink);
  }
  .feedback-panel {
    position: fixed;
    top: calc(var(--sp-2) + 2rem);
    right: var(--sp-2);
    z-index: 30;
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    width: 280px;
    padding: var(--sp-3);
    background: var(--instrument-raised);
    border: var(--edge-w, 1px) solid var(--edge-inst);
    border-radius: var(--radius);
    box-shadow: var(--shadow-lift);
  }
  .feedback-text {
    width: 100%;
    box-sizing: border-box;
    resize: vertical;
    background: var(--instrument);
    color: var(--ink);
    border: var(--edge-w, 1px) solid var(--edge-inst);
    border-radius: var(--radius);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    padding: var(--sp-2);
  }
  .feedback-shot { display: flex; flex-direction: column; gap: var(--sp-1); }
  .feedback-thumb {
    width: 100%;
    border-radius: var(--radius);
    border: var(--edge-w, 1px) solid var(--edge-inst);
  }
  .feedback-link {
    background: none;
    border: 0;
    padding: 0;
    color: var(--ink-dim);
    font-family: var(--font-ui);
    font-size: var(--t-11);
    text-align: left;
    cursor: pointer;
  }
  .feedback-link:hover { color: var(--ink); }
  .feedback-actions { display: flex; align-items: center; justify-content: flex-end; gap: var(--sp-2); }
  .feedback-status { font-size: var(--t-11); margin-right: auto; }
  .feedback-status--ok { color: var(--ink-dim); }
  .feedback-status--error { color: var(--danger); }
  .feedback-cancel,
  .feedback-submit {
    padding: var(--sp-1) var(--sp-3);
    border-radius: var(--radius);
    border: var(--edge-w, 1px) solid var(--edge-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    cursor: pointer;
  }
  .feedback-cancel { background: none; color: var(--ink-dim); }
  .feedback-submit { background: var(--ink); color: var(--felt-sunk); border-color: var(--ink); }
  .feedback-submit:disabled { opacity: 0.5; cursor: default; }
</style>

import { mount } from 'svelte';
import FeedbackButton from './FeedbackButton.svelte';
import { clientBreadcrumbs } from '../lib/breadcrumbs';
import { defaultSettings } from '../lib/playsettings';

clientBreadcrumbs.setView(41, 42);
clientBreadcrumbs.setPlay(defaultSettings(), ['9:trigger:yield']);
clientBreadcrumbs.record('intent_sent', { decision_kind: 'target', choices: [{ index: 3, kind: 'permanent' }] });

(window as unknown as { feedbackForm: Record<string, string> | null }).feedbackForm = null;
window.fetch = async (_input, init) => {
  const form = init?.body as FormData;
  const read = (name: string) => form.get(name);
  (window as unknown as { feedbackForm: Record<string, string> | null }).feedbackForm = {
    text: String(read('text')),
    table: String(read('table')),
    seat: String(read('seat')),
    client: String(read('client.json')),
  };
  return new Response('{}', { status: 201, headers: { 'Content-Type': 'application/json' } });
};

mount(FeedbackButton, { target: document.getElementById('app')!, props: { table: 'table-fixture', seat: 2 } });

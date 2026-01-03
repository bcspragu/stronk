import type { Handle, HandleFetch } from '@sveltejs/kit';
import { SERVER_ENDPOINT, LOCAL_BACKEND_ENDPOINT } from '$env/static/private';

export const handle: Handle = async ({ event, resolve }) => {
	const response = await resolve(event);

	// Prevent bfcache issues where the browser restores a stale snapshot
	// when returning to the tab after navigating away
	if (response.headers.get('content-type')?.includes('text/html')) {
		response.headers.set('cache-control', 'private, no-store');
	}

	return response;
};

export const handleFetch: HandleFetch = async ({ request, fetch }) => {
	if (request.url.startsWith(`${SERVER_ENDPOINT}/`)) {
		// clone the original request, but change the URL
		request = new Request(
			request.url.replace(`${SERVER_ENDPOINT}/`, `${LOCAL_BACKEND_ENDPOINT}/`),
			request
		);
	}

	return fetch(request);
};

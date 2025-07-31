import type { HandleFetch } from '@sveltejs/kit';
import { SERVER_ENDPOINT, LOCAL_BACKEND_ENDPOINT } from '$env/static/private';

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

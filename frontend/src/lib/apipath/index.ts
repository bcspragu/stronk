import { PUBLIC_API_BASE_URL } from '$env/static/public';
import { browser, dev } from '$app/environment';

const apipath = (path: string, params?: Record<string, string>) => {
	const baseURL = browser && !dev ? '' : PUBLIC_API_BASE_URL;
	const query = params ? `?${new URLSearchParams(params)}` : '';
	return `${baseURL}${path}${query}`;
};

export default apipath;

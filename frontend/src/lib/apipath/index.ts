import { PUBLIC_DEV_API_BASE_URL } from '$env/static/public';
import { dev } from '$app/environment';

const apipath = (path: string, params?: Record<string, string>) => {
	const baseURL = dev ? PUBLIC_DEV_API_BASE_URL : '';
	const query = params ? `?${new URLSearchParams(params)}` : '';
	return `${baseURL}${path}${query}`;
};

export default apipath;

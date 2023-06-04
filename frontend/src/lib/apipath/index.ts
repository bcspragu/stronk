import { PUBLIC_API_BASE_URL } from '$env/static/public';
import { browser, dev } from '$app/environment';

const apipath = (path: string) => {
	const baseURL = browser && !dev ? '' : PUBLIC_API_BASE_URL;
	return `${baseURL}${path}`;
}

export default apipath;
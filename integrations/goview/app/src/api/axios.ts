// Network access belongs to the Noyo host; no localStorage token is read here.
export default function disabledRequest<T = any>(..._args: any[]): Promise<T> { return Promise.reject(new Error('Direct upstream HTTP is disabled')) }

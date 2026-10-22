const denied = (..._args: any[]): Promise<any> => Promise.reject(new Error('Direct HTTP and executable request parameters are disabled'))
export const get = denied, post = denied, put = denied, patch = denied, del = denied
export const http = (..._args: any[]) => denied
export const customizeHttp = denied

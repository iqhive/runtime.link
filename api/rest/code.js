let wrap = function (resp) {
  if (resp.status >= 200 && resp.status < 300) {
    if (resp.status === 204) return null;
    return resp.json();
  } else {
    return Promise.reject(resp.body);
  }
};

// A function has properties of its own, such as name and length, that plain
// assignment fails on, and a resource with path values is a function.
let define = function (object, key, value) {
  Object.defineProperty(object, key, { value, enumerable: true, writable: true, configurable: true });
};

let search = function (query) {
  let params = new URLSearchParams(query).toString();
  return params ? `?${params}` : "";
};

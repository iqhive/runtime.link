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

// A path value with nothing in it would make the path of another resource, the
// one without the value, so there is no request to send for it. A list is sent
// with a comma between its elements, and a comma inside an element escaped, as
// that is how the server tells the two apart.
let segment = function (value) {
  if (value === undefined || value === null || `${value}` === "") {
    throw new Error("a path value cannot be empty");
  }
  if (Array.isArray(value)) {
    return value.map((element) => (element === undefined || element === null ? "" : encodeURIComponent(element))).join(",");
  }
  return encodeURIComponent(value);
};

let search = function (query) {
  let params = new URLSearchParams(query).toString();
  return params ? `?${params}` : "";
};

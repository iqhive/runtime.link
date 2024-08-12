let wrap = function (resp) {
  if (resp.status > 200 && resp.status < 300) {
    if (resp.status === 204) return null;
    return resp.json();
  } else {
    return Promise.reject(resp.body);
  }
};

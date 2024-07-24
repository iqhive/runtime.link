(async function () {
  let http = async function (method, accept, path, payload) {
    try {
      let response = await fetch(path, {
        method: method,
        headers: {
          Accept: accept,
          "Content-Type": "application/json",
        },
        body: payload,
      });
      return response.json();
    } catch (error) {
      alert(error);
    }
  };
  try {
    let schema = await http("GET", "application/schema+json", "");
    $("form").jsonForm({
      schema: schema,
      onSubmit: async function (errors, values) {
        let response = await http(
          "POST",
          "application/json",
          "",
          JSON.stringify(values),
        );
        $("pre").text(JSON.stringify(response, null, 2));
        $("pre").css("display", "block");
      },
    });
  } catch {}
  try {
    let resource = await http("GET", "application/json", "");
    $("pre").text(JSON.stringify(resource, null, 2));
    $("pre").css("display", "block");
  } catch {}
})();

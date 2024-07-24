let req = new XMLHttpRequest();
req.onreadystatechange = function () {
  if (req.readyState === 4 && req.status === 200) {
    let schema = JSON.parse(req.responseText);
    $("form").jsonForm({
      schema: schema,
      onSubmit: function (errors, values) {
        $.ajax({
          type: "POST",
          url: "",
          data: JSON.stringify(values),
          contentType: "application/json",
          success: function () {
            alert("Success!");
          },
          error: function () {
            alert("Error: ${xhr.status}");
          },
        });
      },
    });
  }
};
req.open("GET", "", true);
req.setRequestHeader("Accept", "application/schema+json");
req.send();

function getFormData($form){
    const unindexed_array = $form.serializeArray();
    const indexed_array = {};

    $.map(unindexed_array, function(n, i){
        indexed_array[n['name']] = n['value'];
    });
    return indexed_array;
}

function apiSubmit(form, target) {
    $.ajax({
        url: target,
        type: "POST",
        dataType: "json",
        data: JSON.stringify(getFormData($(form))),
        success: function(result){
            console.log(result)
        },
        error: function(xhr, resp, text) {
            console.log(xhr, resp, text);
        }
    })
}

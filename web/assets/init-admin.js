document.addEventListener("alpine:init", () => {
    Alpine.directive("bind-change-set", (el, { expression, value, modifiers }, { evaluate, cleanup }) => {
        const changeSet = evaluate(expression);
        const fieldName = value || el.name;
        const nativeEventName = "csupdate";

        if (el.type === "file") {
            const isSingle = modifiers.includes("single")

            const changeHandler = (e) => {
                changeSet.patch(fieldName, isSingle ? e.target.files[0] : e.target.files);
            };

            const onChangeSetUpdateHandler = (data) => {
                if (!data[fieldName]) {
                    el.value = "";
                }
                el.dispatchEvent(new CustomEvent(nativeEventName, { detail: data[fieldName] }));
            };

            changeSet.listen(onChangeSetUpdateHandler);
            el.addEventListener('change', changeHandler);

            cleanup(() => {
                changeSet.removeListener(onChangeSetUpdateHandler);
                el.removeEventListener('change', changeHandler)
            })
        } else if (el.type === "checkbox") {
            const changeHandler = (e) => {
                changeSet.patch(fieldName, e.target.checked);
            };

            const onChangeSetUpdateHandler = (data) => {
                el.checked = !!data[fieldName];
                el.dispatchEvent(new CustomEvent(nativeEventName, { detail: !!data[fieldName] }));
            };

            changeSet.listen(onChangeSetUpdateHandler);
            el.addEventListener('change', changeHandler)
            el.checked = changeSet.get()[fieldName];

            cleanup(() => {
                changeSet.removeListener(onChangeSetUpdateHandler);
                el.removeEventListener('change', changeHandler)
            })
        } else {
            const keyupHandler = (e) => {
                changeSet.patch(fieldName, e.target.value);
            };

            const onChangeSetUpdateHandler = (data) => {
                el.value = data[fieldName];
                el.dispatchEvent(new CustomEvent(nativeEventName, { detail: data[fieldName] }));
            };

            changeSet.listen(onChangeSetUpdateHandler);
            el.addEventListener('keyup', keyupHandler)
            el.value = changeSet.get()[fieldName];

            cleanup(() => {
                changeSet.removeListener(onChangeSetUpdateHandler);
                el.removeEventListener('keyup', keyupHandler)
            })
        }
    });
});
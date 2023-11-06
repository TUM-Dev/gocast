document.addEventListener("alpine:init", () => {
    const textInputTypes = [
        'text',
        'password',
        'email',
        'search',
        'url',
        'tel',
        'number',
        'datetime-local',
        'date',
        'month',
        'week',
        'time',
        'color'
    ];

    const convert = (modifiers, value) => {
        if (modifiers.includes("int")) {
            return parseInt(value);
        } else if (modifiers.includes("float")) {
            return parseFloat(value);
        }
        return value;
    }

    /**
     * Alpine.js Directive: `x-bind-change-set`
     *
     * This directive allows you to synchronize form elements with a given changeSet object.
     * It is designed to work with different form input types including text inputs,
     * textareas, checkboxes, and file inputs.
     *
     * ## Parameters
     *
     * - `el`: The DOM element this directive is attached to.
     * - `expression`: The JavaScript expression passed to the directive, evaluated to get the changeSet object.
     * - `value`: Optional parameter to specify the field name. Defaults to the `name` attribute of the element.
     * - `modifiers`: Array of additional modifiers to customize the behavior. For instance, use 'single' for single-file uploads.
     * - `evaluate`: Function to evaluate Alpine.js expressions.
     * - `cleanup`: Function to remove event listeners when element is destroyed or directive is unbound.
     *
     * ## Events
     *
     * This directive emits a custom event named "csupdate" whenever the changeSet object or the form element is updated.
     *
     * ## Usage
     *
     * ### Example in HTML
     *
     * ```html
     * <select name="lectureHallId" x-bind-change-set="changeSet">
     *   <option value="0">Self streaming</option>
     *   <!-- ... other options ... -->
     * </select>
     * ```
     *
     * - `changeSet`: The changeSet object you want to bind with the form element.
     *
     * ## Modifiers
     *
     * - `single`: Use this modifier for file inputs when you want to work with a single file instead of a FileList.
     * - `int`: Use this modifier to convert the inserted value to integer.
     * - `float`: Use this modifier to convert the inserted value to float.
     *
     * ```html
     * <input type="file" x-bind-change-set.single="changeSet" />
     * ```
     *
     * ## Notes
     *
     * This directive is intended to be used with the existing `ChangeSet` class.
     * Make sure to import and initialize a `ChangeSet` object in your Alpine.js component
     * to utilize this directive effectively. The `ChangeSet` class should have implemented
     * methods such as `patch`, `listen`, `removeListener`, and `get`,
     * and manage a `DirtyState` object for tracking changes.
     */

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
        } else  if (el.tagName === "textarea" || textInputTypes.includes(el.type)) {
            const keyupHandler = (e) => changeSet.patch(fieldName, convert(modifiers, e.target.value));

            const onChangeSetUpdateHandler = (data) => {
                el.value = `${data[fieldName]}`;
                el.dispatchEvent(new CustomEvent(nativeEventName, { detail: data[fieldName] }));
            };

            changeSet.listen(onChangeSetUpdateHandler);
            el.addEventListener('keyup', keyupHandler)
            el.value = `${changeSet.get()[fieldName]}`;

            cleanup(() => {
                changeSet.removeListener(onChangeSetUpdateHandler);
                el.removeEventListener('keyup', keyupHandler)
            })
        } else {
            const changeHandler = (e) => changeSet.patch(fieldName, convert(modifiers, e.target.value));

            const onChangeSetUpdateHandler = (data) => {
                el.value = `${data[fieldName]}`;
                el.dispatchEvent(new CustomEvent(nativeEventName, { detail: data[fieldName] }));
            };

            changeSet.listen(onChangeSetUpdateHandler);
            el.addEventListener('change', changeHandler)
            el.value = `${changeSet.get()[fieldName]}`;

            cleanup(() => {
                changeSet.removeListener(onChangeSetUpdateHandler);
                el.removeEventListener('change', changeHandler)
            })
        }
    });
});
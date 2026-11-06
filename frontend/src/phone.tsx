export function phoneNumber(value: string) {
  return "+" + value.replace(/\D/g, "");
}
export function formatPhone(value: string) {
  const digits = value.replace(/\D/g, "");
  if (!digits) return "";
  if (digits === "7" || digits === "8") return "+7";
  let national = digits;
  if (value.startsWith("+") || digits.startsWith("8") || digits.length > 10) {
    if (!/^[78]/.test(digits)) return "+7";
    national = digits.slice(1);
  }
  const n = national.slice(0, 10);
  return (
    "+7" +
    (n.length ? "(" + n.slice(0, 3) : "") +
    (n.length >= 3 ? ")" : "") +
    n.slice(3, 6) +
    (n.length > 6 ? "-" + n.slice(6, 8) : "") +
    (n.length > 8 ? "-" + n.slice(8) : "")
  );
}
export function displayPhone(value: string) {
  return /^\+7\d{10}$/.test(phoneNumber(value)) ? formatPhone(value) : value;
}
export function PhoneInput({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <input
      type="tel"
      autoComplete="tel"
      inputMode="tel"
      placeholder="+7(987)654-32-10"
      required
      pattern={"\\+7\\([0-9]{3}\\)[0-9]{3}-[0-9]{2}-[0-9]{2}"}
      title="Введите номер +7 и 10 цифр: +7(987)654-32-10"
      value={value}
      onChange={(event) => {
        const input = event.currentTarget;
        const caret = input.selectionStart ?? input.value.length;
        let raw = input.value;
        let before = raw.slice(0, caret).replace(/\D/g, "").length;
        const deleting =
          (event.nativeEvent as InputEvent).inputType ===
          "deleteContentBackward";
        if (deleting && phoneNumber(raw) === phoneNumber(value) && before > 1) {
          const index = raw.slice(0, caret).search(/\d\D*$/);
          raw = raw.slice(0, index) + raw.slice(index + 1);
          before--;
        }
        const formatted = formatPhone(raw);
        // A national number adds a country-code digit before the typed digits.
        if (
          !raw.startsWith("+") &&
          raw.replace(/\D/g, "").length <= 10 &&
          !raw.replace(/\D/g, "").startsWith("8") &&
          raw.replace(/\D/g, "") !== "7"
        )
          before++;
        let count = 0,
          position = formatted.length;
        for (let i = 0; i < formatted.length; i++) {
          if (/\d/.test(formatted[i])) count++;
          if (count === before) {
            position = i + 1;
            while (
              position < formatted.length &&
              /\D/.test(formatted[position])
            )
              position++;
            break;
          }
        }
        // Set selection synchronously: a queued animation frame can move the
        // caret after the next keystroke when someone types quickly.
        input.value = formatted;
        input.setSelectionRange(position, position);
        onChange(formatted);
      }}
    />
  );
}

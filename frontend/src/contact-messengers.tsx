const messengers = [
  {
    id: "max",
    label: "MAX",
    href: "https://max.ru/u/f9LHodD0cOIhCr6cQVaeSQA4LhkcMxoaO7IRakvnCvEfSosUO3AtwjGvlCw",
  },
  { id: "whatsapp", label: "WhatsApp", href: "https://wa.me/79027625164" },
  { id: "telegram", label: "Telegram", href: "https://t.me/marksv95" },
];
export function ContactMessengers() {
  return (
    <section className="contact-messengers" aria-labelledby="messengers-title">
      <h2 id="messengers-title">Написать нам</h2>
      <div className="messenger-buttons">
        {messengers.map(({ id, label, href }) => (
          <a
            key={id}
            className={`messenger messenger-${id}`}
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Написать в ${label}`}
          >
            <img
              src={`/branding/messengers/${id}.svg`}
              alt=""
              width="26"
              height="26"
            />
            <span>{label}</span>
          </a>
        ))}
      </div>
    </section>
  );
}

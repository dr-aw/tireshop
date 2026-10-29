const steps = [
  {
    title: "Соберите корзину",
    description: "Выберите товары и нужное количество.",
    icon: (
      <>
        <path d="M3 5h4l3 16h15l3-12H8M11 15h14" />
        <circle cx="12" cy="26" r="1.5" />
        <circle cx="24" cy="26" r="1.5" />
        <path d="M16 5v6m-3-3h6" />
      </>
    ),
  },
  {
    title: "Оставьте контакты",
    description: "Имя, телефон и пожелания к заказу.",
    icon: (
      <>
        <rect x="3" y="6" width="26" height="21" rx="4" />
        <circle cx="11" cy="13" r="3" />
        <path d="M6.5 22v-1a4.5 4.5 0 0 1 9 0v1M20 12h5m-5 5h5m-5 5h3" />
      </>
    ),
  },
  {
    title: "Дождитесь звонка",
    description: "Подтвердим наличие и обсудим доставку.",
    icon: (
      <path d="m10 4 4 7-4 3a20 20 0 0 0 8 8l3-4 7 4-2 5c-.5 1-2 1.5-3 1C12 25 7 20 4 9c-.5-1 0-2.5 1-3l5-2ZM20 4a8 8 0 0 1 8 8m-8-3a3 3 0 0 1 3 3" />
    ),
  },
];

export function OrderSteps() {
  return (
    <section
      className="how-to-order"
      id="how-to-order"
      aria-labelledby="order-steps-title"
    >
      <header className="order-steps-heading">
        <p className="eyebrow">ТРИ ПРОСТЫХ ШАГА</p>
        <h2 id="order-steps-title">
          От выбора до заказа<span>.</span>
        </h2>
      </header>
      <ol className="order-steps" role="list">
        {steps.map((step, index) => (
          <li className="order-step" key={step.title}>
            <div className="order-step-visual" aria-hidden="true">
              <span className="order-step-icon">
                <svg
                  viewBox="0 0 32 32"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  {step.icon}
                </svg>
              </span>
              <span className="order-step-number">0{index + 1}</span>
            </div>
            <h3>{step.title}</h3>
            <p>{step.description}</p>
          </li>
        ))}
      </ol>
    </section>
  );
}

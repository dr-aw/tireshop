// Keep the constructor's document-writing script isolated from React's DOM.
const documentHTML = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><style>html,body{margin:0;width:100%;height:400px;overflow:hidden}</style></head><body><script type="text/javascript" charset="utf-8" async src="https://api-maps.yandex.ru/services/constructor/1.0/js/?um=constructor%3Abeb2df27eccf6225a25a9f91aeab436ffe15a4fd72b4e8f4e3c459efd8e4d2c5&amp;width=100%25&amp;height=400&amp;lang=ru_RU&amp;scroll=true"></script></body></html>`;
export function ContactMap() {
  return (
    <div className="contact-map">
      <iframe
        title="Склад tireshop на карте Яндекса"
        srcDoc={documentHTML}
        loading="lazy"
        sandbox="allow-scripts allow-same-origin allow-popups"
        referrerPolicy="strict-origin-when-cross-origin"
      />
    </div>
  );
}

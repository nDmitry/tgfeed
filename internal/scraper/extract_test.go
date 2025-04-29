package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "Bold first line as title",
			html:     `<div class="tgme_widget_message_text js-message_text before_footer" dir="auto"><b>Результаты по основным активам за 20 лет</b><br><br>Обновленные данные, включающие 2024 год, по инфляции, долговым рынкам, валютам, акциям, драгоценным металлам и нескольким модельным портфелям.<br><br><a href="https://capital-gain.ru/posts/2024-assets-results/?utm_source=telegram&amp;amp;utm_medium=messenger&amp;amp;utm_campaign=announce" target="_blank" rel="noopener" onclick="return confirm('Open this link?\n\n'+this.href);">https://capital-gain.ru/posts/2024-assets-results/</a><br><br><a href="?q=%23%D0%B0%D0%B2%D1%82%D0%BE%D1%80%D1%81%D0%BA%D0%B8%D0%B5_%D1%82%D0%B5%D0%BA%D1%81%D1%82%D1%8B">#авторские_тексты</a> <a href="https://t.me/capitalgainru" target="_blank">@capitalgainru</a><span style="display: inline-block; width: 90px;"></span></div>`,
			expected: "Результаты по основным активам за 20 лет",
		},
		{
			name:     "First line with parenthesis exceeding limit",
			html:     `<div class="tgme_widget_message_text js-message_text before_footer" dir="auto">Вчера закрылась просадка стоимости индекса Мосбиржи полной доходности (с учетом дивидендов) <b>MCFTR</b>, длившаяся с 20 октября 2021. Индекс тогда был на уровне <b>8125,8</b> пунктов — это прошлый пик, к 26 сентября 2022 стоимость опустилась до <b>3775,8</b> пунктов (-53,5%) — это дно. 17 февраля 2025, спустя <b>1216 дней</b> от пика, индекс закрылся на отметке <b>8199,7</b>, прибавив со дна +117,5%. Интересно, что это была не самая длинная просадка, и вокруг кризиса 2008 года тоже не самая длинная (1206 дней). Дольше всего MCFTR не мог восстановиться в 2011–2015 годах (1385 дней), хотя тогда и опускался не так глубоко. Индекс ОФЗ RGBITR тоже близок к историческому пику (13.06.2023  было 628,9 пунктов), не хватает ещё нескольких процентов, чтобы его преодолеть.<br><br><a href="https://capital-gain.ru/posts/20991/?utm_source=telegram&amp;amp;utm_medium=messenger&amp;amp;utm_campaign=announce" target="_blank" rel="noopener" onclick="return confirm('Open this link?\n\n'+this.href);">https://capital-gain.ru/posts/20991/</a><br><br><a href="?q=%23%D0%BE%D0%B4%D0%BD%D0%BE%D0%B9_%D1%81%D1%82%D1%80%D0%BE%D0%BA%D0%BE%D0%B9">#одной_строкой</a> <a href="https://t.me/capitalgainru" target="_blank">@capitalgainru</a><span style="display: inline-block; width: 90px;"></span></div>`,
			expected: "Вчера закрылась просадка стоимости индекса Мосбиржи полной доходности…",
		},
		{
			name:     "Long first line with question",
			html:     `<div class="tgme_widget_message_text js-message_text before_footer" dir="auto">А что там с <b>JetLend</b> и аналогами сейчас происходит на фоне повышенной ставки, кто в курсе? Раньше его из всех утюгов рекламировали, а потом все разом перестали, я уж и забыл про него. Какие-то отдельные грустные жалобы изредка попадаются, но на сайте они по-прежнему странный график показывают, что доходность только растет. Напишите, если у вас есть счет, какой процент дефолтов, как изменился за год, что с доходностью?<span style="display: inline-block; width: 90px;"></span></div>`,
			expected: "А что там с JetLend и аналогами сейчас происходит на фоне повышенной ставки…",
		},
		{
			name:     "Sentence ending with question mark",
			html:     `<div class="tgme_widget_message_text js-message_text before_footer" dir="auto">Что случилось с рынком? Это очень длинное предложение, которое должно быть обрезано по первому вопросительному знаку в соответствии с правилами.</div>`,
			expected: "Что случилось с рынком?",
		},
		{
			name:     "Sentence ending with exclamation mark",
			html:     `<div class="tgme_widget_message_text js-message_text before_footer" dir="auto">Внимание! Важная информация о рынке акций, которую нужно знать каждому инвестору в текущей ситуации.</div>`,
			expected: "Внимание!",
		},
		{
			name:     "Title with exactly 80 characters",
			html:     `<div class="tgme_widget_message_text js-message_text before_footer" dir="auto">Ровно восемьдесят символов в этом заголовке чтобы проверить работу без троеточия</div>`,
			expected: "Ровно восемьдесят символов в этом заголовке чтобы проверить работу без троеточия",
		},
		{
			name:     "Title with more than 80 characters, break at word boundary",
			html:     `<div class="tgme_widget_message_text js-message_text before_footer" dir="auto">Этот заголовок длиннее восьмидесяти символов и должен быть обрезан по границе слова не нарушая целостность последнего слова в строке.</div>`,
			expected: "Этот заголовок длиннее восьмидесяти символов и должен быть обрезан по границе…",
		},
		{
			name:     "Title with no spaces under 80 chars",
			html:     `<div class="tgme_widget_message_text js-message_text before_footer" dir="auto">ThisIsAVeryLongWordWithoutAnySpacesOrBreaksToTestHowTheAlgorithmHandlesLongWordsWithoutSpaces</div>`,
			expected: "ThisIsAVeryLongWordWithoutAnySpacesOrBreaksToTestHowTheAlgorithmHandlesLongWord…",
		},
		{
			name:     "Title with no message text",
			html:     `<div class="tgme_widget_message_bubble"></div>`,
			expected: "",
		},
		{
			name:     "Should extract the title out of the inner div",
			html:     `<div class="tgme_widget_message_text js-message_text" dir="auto"><div class="tgme_widget_message_text js-message_text" dir="auto"><b>Стартовали общие OTC-торги заблокированными акциями<br></b><br>Доступны торги для 127 американских акций и ETF Finex на Мосбирже (список в комментариях).</div></div>`,
			expected: "Стартовали общие OTC-торги заблокированными акциями",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock colly HTMLElement with our test HTML
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			require.NoError(t, err)

			selection := doc.Selection

			// Create a colly.HTMLElement with the minimal required fields
			element := &colly.HTMLElement{
				DOM: selection,
			}

			// Call the function under test
			title := extractTitle(element)

			// Assert the result
			assert.Equal(t, tt.expected, title)
		})
	}
}

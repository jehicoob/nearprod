"""Native browser checks of the theme preference and operational type scale."""
from playwright.sync_api import expect


def check_preference(page, context, url, evidence, passed):
    root = page.locator('html')
    picker = page.get_by_role('combobox', name='Tema de apariencia')
    page.emulate_media(color_scheme='dark')
    expect(root).to_have_attribute('data-theme', 'dark')
    expect(picker).to_have_value('system')
    picker.select_option('light')
    page.reload()
    expect(picker).to_have_value('light')
    expect(root).to_have_attribute('data-theme', 'light')
    page.emulate_media(color_scheme='light')
    page.emulate_media(color_scheme='dark')
    expect(root).to_have_attribute('data-theme', 'light')
    picker.select_option('system')
    expect(root).to_have_attribute('data-theme', 'dark')
    page.emulate_media(color_scheme='light')
    expect(root).to_have_attribute('data-theme', 'light')
    other = context.new_page()
    try:
        other.goto(url)
        other.get_by_role('combobox', name='Tema de apariencia').select_option('dark')
        expect(root).to_have_attribute('data-theme', 'dark')
        expect(picker).to_have_value('dark')
    finally:
        other.close()
    page.add_init_script("""new PerformanceObserver(list => {
      if (list.getEntries().some(entry => entry.name === 'first-contentful-paint'))
        document.documentElement.dataset.firstPaintTheme = document.documentElement.dataset.theme;
    }).observe({type:'paint',buffered:true});""")
    page.reload()
    expect(root).to_have_attribute('data-first-paint-theme', 'dark')
    expect(root).to_have_css('background-color', 'rgb(24, 28, 34)')
    page.screenshot(path=str(evidence / 'theme-login-dark.png'))
    picker.select_option('light')
    passed('Native theme: system, explicit preference, reload, cross-tab sync and dark first paint')


def check_mcp_themes(page, evidence, passed):
    picker = page.get_by_role('combobox', name='Tema de apariencia')
    for theme in ['light', 'dark']:
        picker.select_option(theme)
        expect(page.locator('html')).to_have_attribute('data-theme', theme)
        for heading in page.locator('.mcp-grid h2, .panel-heading h2').all():
            expect(heading).to_have_css('font-size', '16px')
        for copy in page.locator('.mcp-steps, .mcp-grid .hints, .mcp-grid p').all():
            expect(copy).to_have_css('font-size', '14px')
        expect(page.locator('.mcp-config')).to_have_css('font-size', '13px')
        panel_color = 'rgb(34, 40, 49)' if theme == 'dark' else 'rgb(255, 255, 255)'
        expect(page.locator('.panel').first).to_have_css('background-color', panel_color)
        for width in [1440, 750, 430, 320]:
            page.set_viewport_size({'width': width, 'height': 1000})
            expect(picker).to_be_visible()
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1'), (theme, width)
            page.screenshot(path=str(evidence / f'mcp-{theme}-{width}.png'), full_page=True)
        # Respect the user's root font setting; reflow instead of hard-coded tiny text.
        page.evaluate("document.documentElement.style.fontSize = '20px'")
        expect(page.locator('.mcp-steps')).to_have_css('font-size', '17.5px')
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1')
        page.evaluate("document.documentElement.style.removeProperty('font-size')")
    page.set_viewport_size({'width': 1440, 'height': 1000})
    # Remaining operational flows run in dark mode to exercise all shared surfaces.
    passed('MCP typography: 16px headings, 14px reading, 13px code; both themes, 320–1440px and larger text')

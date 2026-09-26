"""Build the deployment PDF: python build_guide.py (requires reportlab).

Uses Windows Arial/Consolas or Linux DejaVu fonts with embedded Cyrillic glyphs.
"""
from pathlib import Path
import re
from html import escape

from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, PageBreak, Table, TableStyle, Preformatted,
)

ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / 'output/pdf/SmartQuarter_Server_Deployment_Guide.pdf'
WIDTH, HEIGHT = A4
MARGIN = 44
CONTENT = WIDTH - MARGIN * 2
NAVY = colors.HexColor('#122C43')
TEAL = colors.HexColor('#087F82')
GRAY = colors.HexColor('#526575')


def fonts():
    windows = Path('C:/Windows/Fonts')
    linux = Path('/usr/share/fonts/truetype/dejavu')
    paths = (
        [windows / n for n in ('arial.ttf', 'arialbd.ttf', 'consola.ttf')]
        if windows.exists() else
        [linux / n for n in ('DejaVuSans.ttf', 'DejaVuSans-Bold.ttf', 'DejaVuSansMono.ttf')]
    )
    for name, path in zip(('Body', 'BodyBold', 'Code'), paths):
        pdfmetrics.registerFont(TTFont(name, str(path)))
    pdfmetrics.registerFontFamily('Body', normal='Body', bold='BodyBold', italic='Body', boldItalic='BodyBold')


def inline(text):
    text = re.sub(r'\[([^\]]+)\]\(([^)]+)\)', r'\1', text)
    text = escape(text)
    text = re.sub(r'\*\*(.+?)\*\*', r'<b>\1</b>', text)
    text = re.sub(r'`([^`]+)`', r'<font name="Code">\1</font>', text)
    text = re.sub(r'(https://[^\s<>;]+)', r'<link href="\1" color="#087F82">\1</link>', text)
    return text


def decorate(canvas, doc):
    canvas.saveState()
    canvas.setFillColor(TEAL)
    canvas.rect(0, HEIGHT - 9, WIDTH, 9, fill=1, stroke=0)
    canvas.setFont('BodyBold', 8)
    canvas.setFillColor(GRAY)
    canvas.drawString(MARGIN, HEIGHT - 30, 'SMARTQUARTER  /  РУКОВОДСТВО ПО РАЗВЁРТЫВАНИЮ')
    canvas.setStrokeColor(colors.HexColor('#DAE4E9'))
    canvas.line(MARGIN, 36, WIDTH - MARGIN, 36)
    canvas.setFont('Body', 8)
    canvas.drawString(MARGIN, 23, 'Ubuntu 24.04 LTS  •  Docker Compose  |  26.09.2026')
    canvas.drawRightString(WIDTH - MARGIN, 23, f'{doc.page:02d}')
    canvas.restoreState()


def build():
    fonts()
    normal = ParagraphStyle('body', fontName='Body', fontSize=10.5, leading=15,
                            textColor=NAVY, spaceAfter=9, splitLongWords=True)
    heading = ParagraphStyle('heading', parent=normal, fontName='BodyBold', fontSize=18,
                             leading=22, spaceAfter=17, keepWithNext=True)
    title = ParagraphStyle('title', parent=heading, fontSize=36, leading=42, spaceBefore=26,
                           spaceAfter=22)
    code = ParagraphStyle('code', fontName='Code', fontSize=9, leading=11.5,
                          backColor=colors.HexColor('#F0F5F7'), borderPadding=9,
                          spaceBefore=5, spaceAfter=13, textColor=NAVY)
    cell = ParagraphStyle('cell', parent=normal, fontSize=8.6, leading=11.6, spaceAfter=0)
    bullet = ParagraphStyle('bullet', parent=normal, leftIndent=12, firstLineIndent=-10, spaceAfter=6)
    lines = (ROOT / 'docs/deployment/server-guide.md').read_text(encoding='utf-8').splitlines()
    story = []
    i = 0
    while i < len(lines):
        line = lines[i].strip()
        if not line:
            i += 1
            continue
        if line == '<!-- pagebreak -->':
            story.append(PageBreak())
        elif line.startswith('# '):
            story.append(Paragraph(inline(line[2:]), title))
        elif line.startswith('## '):
            story.append(Paragraph(inline(line[3:]), heading))
        elif line.startswith('```'):
            block = []
            i += 1
            while i < len(lines) and not lines[i].startswith('```'):
                block.append(lines[i])
                i += 1
            # Never silently clip executable commands; shrink only if needed.
            longest = max((pdfmetrics.stringWidth(s, 'Code', 9) for s in block), default=1)
            size = min(9, 9 * (CONTENT - 20) / max(longest, 1))
            if size < 6.8:
                raise ValueError(f'Command line too wide: {max(block, key=len)}')
            style = ParagraphStyle('block', parent=code, fontSize=size)
            story.append(Preformatted('\n'.join(block), style))
        elif line.startswith('|'):
            rows = []
            while i < len(lines) and lines[i].startswith('|'):
                values = [v.strip() for v in lines[i].strip().strip('|').split('|')]
                if not all(re.fullmatch(r'[- :]+', v) for v in values):
                    rows.append([Paragraph(inline(v), cell) for v in values])
                i += 1
            i -= 1
            ratios = [0.30, 0.70] if len(rows[0]) == 2 else [0.25, 0.38, 0.37]
            table = Table(rows, colWidths=[CONTENT * r for r in ratios], repeatRows=1, hAlign='LEFT')
            table.setStyle(TableStyle([
                ('BACKGROUND', (0, 0), (-1, 0), colors.HexColor('#D9EEED')),
                ('ROWBACKGROUNDS', (0, 1), (-1, -1), [colors.white, colors.HexColor('#F3F6F8')]),
                ('VALIGN', (0, 0), (-1, -1), 'TOP'),
                ('LEFTPADDING', (0, 0), (-1, -1), 9), ('RIGHTPADDING', (0, 0), (-1, -1), 9),
                ('TOPPADDING', (0, 0), (-1, -1), 7), ('BOTTOMPADDING', (0, 0), (-1, -1), 7),
                ('LINEBELOW', (0, 0), (-1, 0), 0.7, TEAL),
            ]))
            story.extend([table, Spacer(1, 12)])
        elif line.startswith('- '):
            story.append(Paragraph('• ' + inline(line[2:]), bullet))
        else:
            story.append(Paragraph(inline(line), normal))
        i += 1
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    SimpleDocTemplate(str(OUTPUT), pagesize=A4, rightMargin=MARGIN, leftMargin=MARGIN,
                      topMargin=54, bottomMargin=51, title='Умный Квартал: развёртывание на Ubuntu',
                      author='SmartQuarter', subject='Docker Compose, HTTPS, MAX, CI, backup').build(
        story, onFirstPage=decorate, onLaterPages=decorate)
    print(OUTPUT)


if __name__ == '__main__':
    build()

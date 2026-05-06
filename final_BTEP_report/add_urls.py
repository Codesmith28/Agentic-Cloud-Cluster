import re

with open('General/References.bib', 'r') as f:
    content = f.read()

# Replace url={...} with note={\url{...}}
content = re.sub(r'url=\{(http[^\}]+)\}', r'note={\\url{\1}}', content)

# Add URL to moritz2018ray
content = content.replace('year = {2018}\n}', 'year = {2018},\n  note={\\url{https://arxiv.org/abs/1712.05889}}\n}')

# Add URL to haarnoja2018sac
content = content.replace('year = {2018}\n}', 'year = {2018},\n  note={\\url{https://arxiv.org/abs/1801.01290}}\n}')

# Add URL to verma2015large
content = content.replace('year={2015}\n}', 'year={2015},\n  note={\\url{https://research.google/pubs/pub43438/}}\n}')

# Add URL to rocklin2015dask
content = content.replace('year = {2015}\n}', 'year = {2015},\n  note={\\url{https://conference.scipy.org/proceedings/scipy2015/pdfs/matthew_rocklin.pdf}}\n}')

with open('General/References.bib', 'w') as f:
    f.write(content)


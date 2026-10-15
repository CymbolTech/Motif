'use client';

import { useEffect, useRef, useState } from 'react';
import { emit } from '@/game/store';
import { Coord } from '@/components/ui/Coord';
import { RECIPES } from '@/lib/content';

export function Recipes() {
  const [active, setActive] = useState(0);
  const sectionRef = useRef<HTMLElement>(null);
  const recipe = RECIPES[active];

  const open = (index: number) => {
    setActive(index);
    emit({ id: `recipe-${RECIPES[index].id}`, type: 'recipe.opened', payload: { genre: RECIPES[index].id } });
  };

  // The first recipe counts as opened once it is actually on screen.
  useEffect(() => {
    const section = sectionRef.current;
    if (!section) return;
    const observer = new IntersectionObserver(([entry]) => {
      if (!entry.isIntersecting) return;
      emit({ id: `recipe-${RECIPES[0].id}`, type: 'recipe.opened', payload: { genre: RECIPES[0].id } });
      observer.disconnect();
    }, { threshold: 0.4 });
    observer.observe(section);
    return () => observer.disconnect();
  }, []);

  return (
    <section className="recipes section" id="recipes" aria-labelledby="recipes-title" ref={sectionRef}>
      <Coord />
      <div className="frame section-head">
        <h2 id="recipes-title" className="section-title display">
          Made for products with users.
        </h2>
        <p className="section-lede">The same loop, six genres. Every recipe is a real rule shape from the scenario cookbook.</p>
      </div>

      <div className="frame recipes-body">
        <div className="recipes-list" role="tablist" aria-label="Genres">
          {RECIPES.map((item, index) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              id={`recipe-tab-${item.id}`}
              aria-selected={active === index}
              aria-controls="recipe-panel"
              className="recipes-genre display"
              onClick={() => open(index)}
            >
              {item.genre}
            </button>
          ))}
        </div>

        <div className="recipe" id="recipe-panel" role="tabpanel" aria-labelledby={`recipe-tab-${recipe.id}`} key={recipe.id}>
          <h3 className="recipe-title">{recipe.title}</h3>
          <ol className="recipe-flow">
            <li>
              <span className="mono">When</span>
              <code>{recipe.event}</code>
            </li>
            <li>
              <span className="mono">If · then</span>
              <pre>{recipe.rule}</pre>
            </li>
            <li>
              <span className="mono">Effect</span>
              <p>{recipe.effect}</p>
            </li>
          </ol>
          <p className="recipe-note">{recipe.note}</p>
        </div>
      </div>
    </section>
  );
}

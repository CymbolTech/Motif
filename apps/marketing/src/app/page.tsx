import { Arcade } from '@/components/sections/Arcade';
import { Capabilities } from '@/components/sections/Capabilities';
import { Faq } from '@/components/sections/Faq';
import { Footer } from '@/components/sections/Footer';
import { Hero } from '@/components/sections/Hero';
import { Loop } from '@/components/sections/Loop';
import { Manifesto } from '@/components/sections/Manifesto';
import { PressStart } from '@/components/sections/PressStart';
import { Recipes } from '@/components/sections/Recipes';
import { ScrollWatch } from '@/components/sections/ScrollWatch';
import './landing.css';
import './arcade.css';

export default function LandingPage() {
  return (
    <>
      <main id="main">
        <Hero />
        <Manifesto />
        <Loop />
        <Arcade />
        <Capabilities />
        <Recipes />
        <Faq />
        <PressStart />
      </main>
      <Footer />
      <ScrollWatch />
    </>
  );
}

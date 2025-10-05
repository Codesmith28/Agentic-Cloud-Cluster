# AI-Driven Agentic Scheduler - Website Documentation

## Overview
This website demonstrates the AI-Driven Agentic Scheduler project, a next-generation distributed task scheduling system using AI planning algorithms.

## Files Structure

```
website/
├── index.html          # Main HTML file
├── style.css           # Stylesheet with animations and responsive design
├── script.js           # Interactive JavaScript functionality
└── README.md          # This file
```

## Features

### 1. **Navigation**
- Fixed navbar with smooth scrolling
- Active section highlighting
- Mobile-responsive menu

### 2. **Sections**

#### Home (Hero Section)
- Animated floating cards
- Call-to-action buttons
- Responsive layout

#### System Overview
- Statistics cards with animated counters
- System description

#### Architecture
- Three main components (Master, Planner, Workers)
- Feature lists for each component
- Hover animations

#### Components
- Tabbed interface for:
  - Master Components (8 modules)
  - Planner Algorithms (4 algorithms)
  - Worker Features
- Interactive tab switching

#### **Diagrams** ⭐ NEW
- **System Architecture Diagram**: SVG visualization showing Master, Planner, Workers, and their connections
- **Sequence Diagram**: Step-by-step task flow from submission to completion
- **Data Flow Diagram**: Central hub showing bidirectional data flow
- Interactive features:
  - Click to highlight nodes
  - Hover effects
  - Animated message flows
  - Zoom with Ctrl+Scroll
  - Double-click to reset zoom
  - Pulse animations
  - Color-coded legend

#### Workflow
- 6-step timeline with detailed explanations
- Code snippets showing API calls
- Visual progression markers

#### Advantages
- Comparison with Kubernetes scheduler
- Benefits grid (4 key benefits)
- Feature highlights

#### Technology Stack
- 6 core technologies with descriptions
- Hover effects

### 3. **Interactive Elements**

#### Diagram Interactivity:
- **Hover Effects**: Elements highlight on hover
- **Click Highlighting**: Click nodes to emphasize them
- **Pulse Animation**: Data flow paths pulse every 3 seconds
- **Zoom Functionality**: 
  - Hold Ctrl + Scroll to zoom in/out
  - Double-click to reset zoom
- **Animated Entry**: Nodes and connections appear with staggered animations
- **Color Legend**: Shows what each color represents

#### Other Interactions:
- Smooth scroll navigation
- Tab switching for components
- Fade-in animations on scroll
- Counter animations for statistics
- 3D tilt effect on architecture cards
- Parallax effect on hero section
- Typing effect on hero title

## Design Features

### Color Scheme
- Primary: `#6366f1` (Indigo)
- Secondary: `#8b5cf6` (Purple)
- Accent: `#ec4899` (Pink)
- Success: `#10b981` (Green)
- Warning: `#f59e0b` (Orange)
- Background: `#0f172a` (Dark slate)
- Cards: `#1e293b` (Slate)

### Animations
- Floating cards animation
- Fade-in on scroll using Intersection Observer
- Counter animations
- Node appearance animations
- Message slide animations
- Pulse effects on data flows
- Gradient transitions

### Responsive Design
- Mobile-first approach
- Breakpoints at 968px and 640px
- Collapsible navigation
- Stacked layouts on mobile
- Responsive diagrams that scale properly

## Browser Support
- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)
- Mobile browsers (iOS Safari, Chrome Mobile)

## Customization

### Colors
Edit CSS variables in `style.css`:
```css
:root {
    --primary-color: #6366f1;
    --secondary-color: #8b5cf6;
    /* ... more variables */
}
```

### Diagrams
- Edit SVG elements in `index.html`
- Modify coordinates in `viewBox` attribute
- Change colors using `stroke` and `fill` attributes
- Add/remove nodes by duplicating `<g>` elements

### Content
- Edit text directly in `index.html`
- Update section descriptions
- Modify component lists
- Add/remove workflow steps

## Performance Optimizations
- Minimal external dependencies
- Optimized animations with CSS transforms
- Lazy loading of animations (on scroll)
- Efficient event listeners
- Debounced scroll events

## Future Enhancements
- [ ] Add real-time metrics dashboard
- [ ] Interactive demo/simulator
- [ ] Dark/Light theme toggle
- [ ] Export diagrams as PNG/SVG
- [ ] More detailed component documentation
- [ ] Video walkthrough
- [ ] API documentation section

## Usage

### Local Development
1. Open `index.html` in a web browser
2. No build process required
3. All files are self-contained

### Deployment
- Can be hosted on any static hosting service:
  - GitHub Pages
  - Netlify
  - Vercel
  - AWS S3
  - Azure Static Web Apps

### Tips for Viewing
1. **Best Experience**: Use a desktop browser with a screen width > 1200px
2. **Diagrams**: 
   - Scroll slowly through the diagrams section to see animations
   - Try Ctrl+Scroll to zoom
   - Click on diagram nodes to highlight them
3. **Performance**: Disable animations if experiencing lag (edit CSS to remove animations)

## Accessibility
- Semantic HTML elements
- ARIA labels on interactive elements
- Keyboard navigation support
- High contrast color scheme
- Scalable text and elements

## Credits
- Fonts: [Inter](https://fonts.google.com/specimen/Inter) from Google Fonts
- Icons: Emoji (no external dependencies)
- Diagrams: Custom SVG graphics

## License
Part of the AI-Driven Agentic Scheduler project  
Cloud Computing Course - 7th Semester

---

**Note**: The diagrams are fully interactive! Try clicking, hovering, and scrolling while holding Ctrl to explore different features.

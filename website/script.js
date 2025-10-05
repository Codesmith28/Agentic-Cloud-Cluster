// Smooth scrolling for navigation links
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function (e) {
        e.preventDefault();
        const target = document.querySelector(this.getAttribute('href'));
        if (target) {
            const offsetTop = target.offsetTop - 80;
            window.scrollTo({
                top: offsetTop,
                behavior: 'smooth'
            });
        }
    });
});

// Active navigation link on scroll
const sections = document.querySelectorAll('section');
const navLinks = document.querySelectorAll('.nav-link');

window.addEventListener('scroll', () => {
    let current = '';
    sections.forEach(section => {
        const sectionTop = section.offsetTop;
        const sectionHeight = section.clientHeight;
        if (window.pageYOffset >= sectionTop - 200) {
            current = section.getAttribute('id');
        }
    });

    navLinks.forEach(link => {
        link.classList.remove('active');
        if (link.getAttribute('href') === `#${current}`) {
            link.classList.add('active');
        }
    });
});

// Tab functionality
const tabButtons = document.querySelectorAll('.tab-btn');
const tabPanes = document.querySelectorAll('.tab-pane');

tabButtons.forEach(button => {
    button.addEventListener('click', () => {
        const targetTab = button.getAttribute('data-tab');

        // Remove active class from all buttons and panes
        tabButtons.forEach(btn => btn.classList.remove('active'));
        tabPanes.forEach(pane => pane.classList.remove('active'));

        // Add active class to clicked button and corresponding pane
        button.classList.add('active');
        document.getElementById(targetTab).classList.add('active');
    });
});

// Mobile menu toggle
const mobileMenuToggle = document.querySelector('.mobile-menu-toggle');
const navMenu = document.querySelector('.nav-menu');

if (mobileMenuToggle) {
    mobileMenuToggle.addEventListener('click', () => {
        navMenu.classList.toggle('active');
        mobileMenuToggle.classList.toggle('active');
    });
}

// Intersection Observer for fade-in animations
const observerOptions = {
    threshold: 0.1,
    rootMargin: '0px 0px -100px 0px'
};

const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            entry.target.classList.add('fade-in');
        }
    });
}, observerOptions);

// Observe all cards and timeline items
document.querySelectorAll('.arch-card, .component-card, .timeline-item, .benefit-card, .tech-card').forEach(el => {
    observer.observe(el);
});

// Add fade-in animation styles dynamically
const style = document.createElement('style');
style.textContent = `
    .fade-in {
        animation: fadeInUp 0.6s ease-out forwards;
    }
    
    @keyframes fadeInUp {
        from {
            opacity: 0;
            transform: translateY(30px);
        }
        to {
            opacity: 1;
            transform: translateY(0);
        }
    }
    
    .arch-card, .component-card, .timeline-item, .benefit-card, .tech-card {
        opacity: 0;
    }
`;
document.head.appendChild(style);

// Navbar background change on scroll
const navbar = document.querySelector('.navbar');
window.addEventListener('scroll', () => {
    if (window.scrollY > 100) {
        navbar.style.background = 'rgba(15, 23, 42, 0.98)';
        navbar.style.boxShadow = '0 4px 20px rgba(0, 0, 0, 0.3)';
    } else {
        navbar.style.background = 'rgba(15, 23, 42, 0.95)';
        navbar.style.boxShadow = 'none';
    }
});

// Floating cards animation randomization
const floatingCards = document.querySelectorAll('.floating-card');
floatingCards.forEach((card, index) => {
    const randomDuration = 2.5 + Math.random() * 2;
    const randomDelay = Math.random() * 2;
    card.style.animationDuration = `${randomDuration}s`;
    card.style.animationDelay = `${randomDelay}s`;
});

// Add parallax effect to hero section
window.addEventListener('scroll', () => {
    const scrolled = window.pageYOffset;
    const hero = document.querySelector('.hero');
    if (hero && scrolled < window.innerHeight) {
        hero.style.transform = `translateY(${scrolled * 0.5}px)`;
        hero.style.opacity = 1 - (scrolled / window.innerHeight);
    }
});

// Code snippet syntax highlighting (simple version)
document.querySelectorAll('.code-snippet code').forEach(code => {
    code.innerHTML = code.textContent.replace(/→/g, '<span style="color: #ec4899;">→</span>');
    code.innerHTML = code.innerHTML.replace(/:/g, '<span style="color: #8b5cf6;">:</span>');
});

// Add typing effect to hero title
const heroTitle = document.querySelector('.hero-title');
if (heroTitle) {
    const text = heroTitle.textContent;
    heroTitle.textContent = '';
    let i = 0;
    
    function typeWriter() {
        if (i < text.length) {
            heroTitle.textContent += text.charAt(i);
            i++;
            setTimeout(typeWriter, 50);
        }
    }
    
    // Start typing effect after a short delay
    setTimeout(typeWriter, 500);
}

// Counter animation for stats
function animateCounter(element, target, duration) {
    let start = 0;
    const increment = target / (duration / 16);
    
    function updateCounter() {
        start += increment;
        if (start < target) {
            element.textContent = Math.ceil(start);
            requestAnimationFrame(updateCounter);
        } else {
            element.textContent = target;
        }
    }
    
    updateCounter();
}

// Observe stats section for counter animation
const statsObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            const statNumbers = entry.target.querySelectorAll('.stat-number');
            statNumbers.forEach((stat, index) => {
                const value = stat.textContent;
                if (value !== '∞') {
                    stat.textContent = '0';
                    animateCounter(stat, parseInt(value), 1500);
                }
            });
            statsObserver.unobserve(entry.target);
        }
    });
}, { threshold: 0.5 });

const statsGrid = document.querySelector('.stats-grid');
if (statsGrid) {
    statsObserver.observe(statsGrid);
}

// Add hover effect for architecture cards with tilt
document.querySelectorAll('.arch-card').forEach(card => {
    card.addEventListener('mousemove', (e) => {
        const rect = card.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        
        const centerX = rect.width / 2;
        const centerY = rect.height / 2;
        
        const rotateX = (y - centerY) / 10;
        const rotateY = (centerX - x) / 10;
        
        card.style.transform = `perspective(1000px) rotateX(${rotateX}deg) rotateY(${rotateY}deg) translateY(-5px)`;
    });
    
    card.addEventListener('mouseleave', () => {
        card.style.transform = 'perspective(1000px) rotateX(0) rotateY(0) translateY(0)';
    });
});

// Dynamic gradient background effect
const hero = document.querySelector('.hero');
if (hero) {
    document.addEventListener('mousemove', (e) => {
        const x = (e.clientX / window.innerWidth) * 100;
        const y = (e.clientY / window.innerHeight) * 100;
        
        hero.style.background = `
            radial-gradient(circle at ${x}% ${y}%, rgba(99, 102, 241, 0.2), transparent 50%),
            radial-gradient(ellipse at top, rgba(99, 102, 241, 0.15), transparent),
            radial-gradient(ellipse at bottom, rgba(139, 92, 246, 0.15), transparent)
        `;
    });
}

// Log page load completion
console.log('%c🤖 AI-Driven Agentic Scheduler Website Loaded Successfully!', 'color: #6366f1; font-size: 16px; font-weight: bold;');
console.log('%cExplore the future of distributed task scheduling', 'color: #8b5cf6; font-size: 12px;');

// Diagram Interactivity
document.addEventListener('DOMContentLoaded', () => {
    // Add tooltips to diagram nodes
    const diagramNodes = document.querySelectorAll('.diagram-node');
    
    diagramNodes.forEach(node => {
        node.addEventListener('mouseenter', function() {
            this.style.opacity = '0.8';
        });
        
        node.addEventListener('mouseleave', function() {
            this.style.opacity = '1';
        });
    });
    
    // Animate sequence diagram messages on scroll
    const sequenceDiagram = document.querySelector('.sequence-diagram');
    if (sequenceDiagram) {
        const observerSeq = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    const messages = entry.target.querySelectorAll('.message');
                    messages.forEach((msg, index) => {
                        setTimeout(() => {
                            msg.style.opacity = '1';
                            msg.style.transform = 'translateX(0)';
                        }, index * 200);
                    });
                    observerSeq.unobserve(entry.target);
                }
            });
        }, { threshold: 0.3 });
        
        observerSeq.observe(sequenceDiagram);
    }
    
    // Add click-to-highlight for diagram components
    const architectureDiagram = document.querySelector('.architecture-diagram');
    if (architectureDiagram) {
        const nodes = architectureDiagram.querySelectorAll('.diagram-node');
        
        nodes.forEach(node => {
            node.addEventListener('click', function() {
                // Remove highlight from all nodes
                nodes.forEach(n => {
                    const rect = n.querySelector('rect');
                    if (rect) {
                        rect.style.stroke = rect.getAttribute('stroke');
                        rect.style.strokeWidth = '2';
                    }
                });
                
                // Highlight clicked node
                const rect = this.querySelector('rect');
                if (rect) {
                    rect.style.stroke = '#ec4899';
                    rect.style.strokeWidth = '4';
                }
            });
        });
    }
    
    // Pulse animation for data flow diagram
    const dataflowDiagram = document.querySelector('.dataflow-diagram');
    if (dataflowDiagram) {
        setInterval(() => {
            const paths = dataflowDiagram.querySelectorAll('path[stroke]');
            paths.forEach((path, index) => {
                setTimeout(() => {
                    path.style.strokeWidth = '3';
                    path.style.opacity = '1';
                    setTimeout(() => {
                        path.style.strokeWidth = '2';
                        path.style.opacity = '0.8';
                    }, 300);
                }, index * 100);
            });
        }, 3000);
    }
    
    // Diagram zoom functionality
    const diagrams = document.querySelectorAll('.architecture-diagram, .sequence-diagram, .dataflow-diagram');
    diagrams.forEach(diagram => {
        let scale = 1;
        
        diagram.addEventListener('wheel', (e) => {
            if (e.ctrlKey) {
                e.preventDefault();
                const delta = e.deltaY > 0 ? -0.1 : 0.1;
                scale = Math.max(0.5, Math.min(2, scale + delta));
                diagram.style.transform = `scale(${scale})`;
                diagram.style.transformOrigin = 'center';
            }
        });
        
        // Reset on double-click
        diagram.addEventListener('dblclick', () => {
            scale = 1;
            diagram.style.transform = 'scale(1)';
        });
    });
});

// Add legend for diagrams
function createDiagramLegend() {
    const diagramContainers = document.querySelectorAll('.diagram-container');
    
    diagramContainers.forEach((container, index) => {
        if (index === 0) { // Only for architecture diagram
            const legend = document.createElement('div');
            legend.className = 'diagram-legend';
            legend.innerHTML = `
                <div class="legend-item">
                    <span class="legend-color" style="background: #6366f1;"></span>
                    <span>Master Node</span>
                </div>
                <div class="legend-item">
                    <span class="legend-color" style="background: #8b5cf6;"></span>
                    <span>Planner Service</span>
                </div>
                <div class="legend-item">
                    <span class="legend-color" style="background: #ec4899;"></span>
                    <span>Worker Nodes</span>
                </div>
                <div class="legend-item">
                    <span class="legend-color" style="background: #10b981;"></span>
                    <span>Heartbeat/Monitor</span>
                </div>
            `;
            container.appendChild(legend);
        }
    });
}

// Add legend styling
const legendStyle = document.createElement('style');
legendStyle.textContent = `
    .diagram-legend {
        display: flex;
        justify-content: center;
        gap: 2rem;
        margin-top: 2rem;
        padding: 1rem;
        background: rgba(99, 102, 241, 0.05);
        border-radius: 8px;
        flex-wrap: wrap;
    }
    
    .legend-item {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        font-size: 0.9rem;
        color: var(--text-secondary);
    }
    
    .legend-color {
        width: 20px;
        height: 20px;
        border-radius: 4px;
        display: inline-block;
    }
`;
document.head.appendChild(legendStyle);

// Initialize legend
setTimeout(createDiagramLegend, 500);

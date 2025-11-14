#!/usr/bin/env python3
"""
CPU Process 10: Numerical Methods and Differential Equations
Tests CPU with numerical integration and ODE solving.
"""
import os
import time
import random
import psutil
import json
import math
from datetime import datetime

def euler_method(f, y0, t0, tf, n):
    """Euler method for solving ODEs"""
    h = (tf - t0) / n
    t = t0
    y = y0
    
    trajectory = [(t, y)]
    
    for _ in range(n):
        y = y + h * f(t, y)
        t = t + h
        trajectory.append((t, y))
    
    return trajectory

def runge_kutta_4(f, y0, t0, tf, n):
    """4th order Runge-Kutta method"""
    h = (tf - t0) / n
    t = t0
    y = y0
    
    trajectory = [(t, y)]
    
    for _ in range(n):
        k1 = h * f(t, y)
        k2 = h * f(t + h/2, y + k1/2)
        k3 = h * f(t + h/2, y + k2/2)
        k4 = h * f(t + h, y + k3)
        
        y = y + (k1 + 2*k2 + 2*k3 + k4) / 6
        t = t + h
        trajectory.append((t, y))
    
    return trajectory

def trapezoidal_integration(f, a, b, n):
    """Trapezoidal rule for numerical integration"""
    h = (b - a) / n
    result = 0.5 * (f(a) + f(b))
    
    for i in range(1, n):
        x = a + i * h
        result += f(x)
    
    return result * h

def simpson_integration(f, a, b, n):
    """Simpson's rule for numerical integration"""
    if n % 2 == 1:
        n += 1  # Make it even
    
    h = (b - a) / n
    result = f(a) + f(b)
    
    for i in range(1, n):
        x = a + i * h
        if i % 2 == 0:
            result += 2 * f(x)
        else:
            result += 4 * f(x)
    
    return result * h / 3

def newton_raphson(f, df, x0, tolerance=1e-6, max_iter=100):
    """Newton-Raphson method for finding roots"""
    x = x0
    iterations = []
    
    for i in range(max_iter):
        fx = f(x)
        dfx = df(x)
        
        if abs(dfx) < 1e-10:
            break
        
        x_new = x - fx / dfx
        iterations.append((i, x, fx))
        
        if abs(x_new - x) < tolerance:
            return x_new, iterations
        
        x = x_new
    
    return x, iterations

def bisection_method(f, a, b, tolerance=1e-6, max_iter=100):
    """Bisection method for finding roots"""
    if f(a) * f(b) >= 0:
        return None, []
    
    iterations = []
    
    for i in range(max_iter):
        c = (a + b) / 2
        fc = f(c)
        
        iterations.append((i, c, fc))
        
        if abs(fc) < tolerance or (b - a) / 2 < tolerance:
            return c, iterations
        
        if f(a) * fc < 0:
            b = c
        else:
            a = c
    
    return (a + b) / 2, iterations

def gradient_descent_1d(f, df, x0, learning_rate=0.01, max_iter=1000):
    """Gradient descent for 1D optimization"""
    x = x0
    trajectory = [(x, f(x))]
    
    for i in range(max_iter):
        gradient = df(x)
        x_new = x - learning_rate * gradient
        
        trajectory.append((x_new, f(x_new)))
        
        if abs(x_new - x) < 1e-6:
            break
        
        x = x_new
    
    return x, trajectory

def jacobi_iteration(A, b, x0, max_iter=100, tolerance=1e-6):
    """Jacobi iteration for solving linear systems"""
    n = len(b)
    x = x0[:]
    
    for iteration in range(max_iter):
        x_new = [0] * n
        
        for i in range(n):
            sum_val = sum(A[i][j] * x[j] for j in range(n) if j != i)
            x_new[i] = (b[i] - sum_val) / A[i][i] if A[i][i] != 0 else 0
        
        # Check convergence
        error = math.sqrt(sum((x_new[i] - x[i])**2 for i in range(n)))
        
        x = x_new
        
        if error < tolerance:
            return x, iteration + 1
    
    return x, max_iter

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 10: Numerical Methods")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-10",
        "odes_solved": 0,
        "integrals_computed": 0,
        "roots_found": 0,
        "optimizations_completed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    numerical_results = []
    
    try:
        # Phase 1: Solving ODEs
        print(f"Phase 1: Solving differential equations...")
        
        # Define various ODEs
        ode_problems = [
            (lambda t, y: y, "exponential growth", 1.0, 0, 2),
            (lambda t, y: -2*y, "exponential decay", 1.0, 0, 3),
            (lambda t, y: t**2 - y, "inhomogeneous", 0.0, 0, 5),
            (lambda t, y: math.sin(t) - y, "driven oscillator", 0.0, 0, 10),
        ]
        
        num_ode_tests = random.randint(15, 30)
        
        for i in range(num_ode_tests):
            f, name, y0, t0, tf = random.choice(ode_problems)
            n_steps = random.randint(500, 2000)
            
            # Solve with Euler method
            euler_start = time.time()
            euler_solution = euler_method(f, y0, t0, tf, n_steps)
            euler_time = time.time() - euler_start
            
            # Solve with RK4
            rk4_start = time.time()
            rk4_solution = runge_kutta_4(f, y0, t0, tf, n_steps)
            rk4_time = time.time() - rk4_start
            
            stats["odes_solved"] += 2
            
            numerical_results.append({
                'problem': 'ODE',
                'type': name,
                'steps': n_steps,
                'euler_time': euler_time,
                'rk4_time': rk4_time,
                'final_euler': euler_solution[-1][1],
                'final_rk4': rk4_solution[-1][1]
            })
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  ODE {i+1}/{num_ode_tests}: {name}, Steps={n_steps}, "
                      f"Euler={euler_time:.3f}s, RK4={rk4_time:.3f}s, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.02, 0.06))
        
        # Phase 2: Numerical Integration
        print(f"\nPhase 2: Computing numerical integrals...")
        
        # Define test functions
        test_functions = [
            (lambda x: x**2, 0, 1, "x^2"),
            (lambda x: math.sin(x), 0, math.pi, "sin(x)"),
            (lambda x: math.exp(-x**2), -2, 2, "exp(-x^2)"),
            (lambda x: 1/(1 + x**2), 0, 1, "1/(1+x^2)"),
            (lambda x: math.sqrt(x) if x >= 0 else 0, 0, 4, "sqrt(x)"),
        ]
        
        num_integral_tests = random.randint(20, 40)
        
        for i in range(num_integral_tests):
            f, a, b, name = random.choice(test_functions)
            n_intervals = random.randint(1000, 5000)
            
            # Trapezoidal rule
            trap_result = trapezoidal_integration(f, a, b, n_intervals)
            
            # Simpson's rule
            simp_result = simpson_integration(f, a, b, n_intervals)
            
            stats["integrals_computed"] += 2
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Integration {i+1}/{num_integral_tests}: ∫{name} = "
                      f"Trap:{trap_result:.6f}, Simp:{simp_result:.6f}, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 3: Root finding
        print(f"\nPhase 3: Finding roots of equations...")
        
        root_problems = [
            (lambda x: x**2 - 4, lambda x: 2*x, 1.0, "x^2 - 4"),
            (lambda x: math.cos(x) - x, lambda x: -math.sin(x) - 1, 0.5, "cos(x) - x"),
            (lambda x: x**3 - 2*x - 5, lambda x: 3*x**2 - 2, 2.0, "x^3 - 2x - 5"),
            (lambda x: math.exp(x) - 3*x, lambda x: math.exp(x) - 3, 1.0, "exp(x) - 3x"),
        ]
        
        num_root_tests = random.randint(15, 30)
        
        for i in range(num_root_tests):
            f, df, x0, name = random.choice(root_problems)
            
            # Newton-Raphson
            try:
                nr_root, nr_iters = newton_raphson(f, df, x0)
                stats["roots_found"] += 1
                
                numerical_results.append({
                    'problem': 'root_finding',
                    'method': 'newton_raphson',
                    'equation': name,
                    'root': nr_root,
                    'iterations': len(nr_iters)
                })
            except:
                pass
            
            # Bisection method (need to find appropriate interval)
            try:
                a, b = x0 - 2, x0 + 2
                if f(a) * f(b) < 0:
                    bis_root, bis_iters = bisection_method(f, a, b)
                    if bis_root is not None:
                        stats["roots_found"] += 1
            except:
                pass
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Root finding {i+1}/{num_root_tests}: {name}, "
                      f"Root ≈ {nr_root:.6f}, Iterations: {len(nr_iters)}, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 4: Optimization
        print(f"\nPhase 4: Optimization problems...")
        
        opt_functions = [
            (lambda x: x**2 - 4*x + 4, lambda x: 2*x - 4, 0.0, "(x-2)^2"),
            (lambda x: x**4 - 3*x**2 + 2*x, lambda x: 4*x**3 - 6*x + 2, 1.0, "x^4 - 3x^2 + 2x"),
            (lambda x: abs(x - 3), lambda x: 1 if x > 3 else -1, 0.0, "|x - 3|"),
        ]
        
        num_opt_tests = random.randint(20, 40)
        
        for i in range(num_opt_tests):
            f, df, x0, name = random.choice(opt_functions)
            learning_rate = random.uniform(0.001, 0.1)
            max_iter = random.randint(500, 2000)
            
            optimal_x, trajectory = gradient_descent_1d(f, df, x0, learning_rate, max_iter)
            
            stats["optimizations_completed"] += 1
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Optimization {i+1}/{num_opt_tests}: {name}, "
                      f"Minimum at x={optimal_x:.4f}, f(x)={f(optimal_x):.4f}, "
                      f"Steps: {len(trajectory)}, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 5: Linear systems
        print(f"\nPhase 5: Solving linear systems...")
        
        num_linear_tests = random.randint(10, 20)
        
        for i in range(num_linear_tests):
            n = random.randint(10, 50)
            
            # Generate diagonally dominant matrix for convergence
            A = [[0.0] * n for _ in range(n)]
            for j in range(n):
                for k in range(n):
                    A[j][k] = random.uniform(-1, 1)
                # Make diagonally dominant
                A[j][j] = sum(abs(A[j][k]) for k in range(n) if k != j) + random.uniform(1, 5)
            
            # Generate random b vector
            b = [random.uniform(-10, 10) for _ in range(n)]
            
            # Initial guess
            x0 = [0.0] * n
            
            # Solve with Jacobi iteration
            solution, iterations = jacobi_iteration(A, b, x0, max_iter=500)
            
            if (i + 1) % 3 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Linear system {i+1}/{num_linear_tests}: Size {n}x{n}, "
                      f"Iterations: {iterations}, CPU: {cpu_percent:.1f}%, "
                      f"Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.05, 0.15))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/numerical_results.json", 'w') as f:
            json.dump(numerical_results[:100], f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"ODEs solved: {stats['odes_solved']}")
        print(f"Integrals computed: {stats['integrals_computed']}")
        print(f"Roots found: {stats['roots_found']}")
        print(f"Optimizations completed: {stats['optimizations_completed']}")
        print(f"Peak CPU: {stats['peak_cpu_percent']:.1f}%")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()

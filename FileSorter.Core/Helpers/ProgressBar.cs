using System.Globalization;

namespace FileSorter.Core.Helpers;

public class ProgressBar : IDisposable
{
    private readonly int _total;
    private readonly string _description;
    private int _current;
    private readonly object _lock = new();
    private bool _disposed;

    public ProgressBar(int total, string description = "Processing")
    {
        _total = total;
        _description = description;
        _current = 0;
        
        Console.CursorVisible = false;
        Draw();
    }

    private void Draw()
    {
        // Save current cursor position
        var currentLeft = Console.CursorLeft;
        var currentTop = Console.CursorTop;

        // Calculate progress
        var percentage = _total == 0 ? 100 : (int)((double)_current / _total * 100);
        var progressBarWidth = 40;
        var filledWidth = (int)((double)_current / _total * progressBarWidth);

        // Build progress bar
        var progressBar = "[" + new string('█', filledWidth) + new string('░', progressBarWidth - filledWidth) + "]";
        
        // Format the line
        var line = $"\r{_description}... {progressBar} {percentage}% ({_current}/{_total})";
        
        // Clear the line and write new content
        Console.Write(new string(' ', Console.WindowWidth - 1));
        Console.Write($"\r{line}");
    }
    
    public void Complete()
    {
        lock (_lock)
        {
            if (_disposed) return;
            
            _current = _total;
            Draw();
            Console.WriteLine(); // Move to next line when complete
        }
    }
    
    public void Update(int current)
    {
        lock (_lock)
        {
            if (_disposed) return;
            
            _current = Math.Min(current, _total);
            Draw();
        }
    }
    
    public void Increment()
    {
        lock (_lock)
        {
            if (_disposed) return;
            
            _current = Math.Min(_current + 1, _total);
            Draw();
        }
    }
    
    public void Dispose()
    {
        lock (_lock)
        {
            if (_disposed) return;
            
            _disposed = true;
            Console.CursorVisible = true;
        }
    }
}

public static class ProgressBarExtensions
{
    public static async Task<T> WithProgressBar<T>(
        this Task<T> task, 
        string description, 
        CancellationToken cancellationToken = default)
    {
        using var progressBar = new ProgressBar(1, description);
        
        var result = await task;
        progressBar.Complete();
        
        return result;
    }
}
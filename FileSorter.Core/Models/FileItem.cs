namespace FileSorter.Core.Models;

public class FileItem
{
    public string Name { get; set; }
    
    public string Extension { get; set; }
    
    public string FullPath { get; set; }
    
    public long Size { get; set; }
    
    public string? Directory { get; set; }
    
    public DateTime LastModifiedDate { get; set; }
    
    public DateTime CreationDate { get; set; }
    
    public FileAttributes Attributes { get; set; }
    
    public bool IsHidden => Attributes.HasFlag(FileAttributes.Hidden);
    
    public bool IsReadOnly => Attributes.HasFlag(FileAttributes.ReadOnly);
    
    public bool IsSystemFile => Attributes.HasFlag(FileAttributes.System);
    
    public FileCategory Category => GetFileCategory(Extension);
    
    public FileItem()
    {
    }
    
    public FileItem(FileInfo fileInfo)
    {
        ArgumentNullException.ThrowIfNull(fileInfo);

        Name = fileInfo.Name;
        FullPath = fileInfo.FullName;
        Extension = fileInfo.Extension;
        Size = fileInfo.Length;
        CreationDate = fileInfo.CreationTime;
        LastModifiedDate = fileInfo.LastWriteTime;
        Directory = fileInfo.DirectoryName;
        Attributes = fileInfo.Attributes;
    }
    
    private static FileCategory GetFileCategory(string extension)
    {
        return extension?.ToLower() switch
        {
            ".txt" or ".doc" or ".docx" or ".pdf" or ".rtf" or ".odt" => FileCategory.Document,
            ".xls" or ".xlsx" or ".csv" or ".ods" => FileCategory.Spreadsheet,
            ".ppt" or ".pptx" or ".odp" => FileCategory.Presentation,
            ".jpg" or ".jpeg" or ".png" or ".gif" or ".bmp" or ".svg" or ".webp" or ".tiff" => FileCategory.Image,
            ".mp3" or ".wav" or ".flac" or ".aac" or ".ogg" or ".wma" => FileCategory.Audio,
            ".mp4" or ".avi" or ".mov" or ".wmv" or ".flv" or ".webm" or ".mkv" => FileCategory.Video,
            ".zip" or ".rar" or ".7z" or ".tar" or ".gz" or ".bz2" => FileCategory.Archive,
            ".exe" or ".msi" or ".deb" or ".rpm" or ".dmg" => FileCategory.Executable,
            ".html" or ".htm" or ".css" or ".js" or ".php" or ".asp" or ".jsp" => FileCategory.Web,
            ".c" or ".cpp" or ".cs" or ".java" or ".py" or ".js" or ".php" or ".rb" or ".go" => FileCategory.Code,
            ".ttf" or ".otf" or ".woff" or ".woff2" => FileCategory.Font,
            _ => FileCategory.Other
        };
    }
}
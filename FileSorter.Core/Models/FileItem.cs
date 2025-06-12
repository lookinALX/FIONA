namespace FileSorter.Core.Models;

/// <summary>
/// Represents a file with its metadata and properties
/// </summary>
public class FileItem
{
    /// <summary>
    /// File name with extension
    /// </summary>
    public string Name { get; set; }
    
    /// <summary>
    /// File extension (including the dot, e.g., ".txt")
    /// </summary>
    public string Extension { get; set; }
    
    /// <summary>
    /// Full path to the file
    /// </summary>
    public string FullPath { get; set; }
    
    /// <summary>
    /// File size in bytes
    /// </summary>
    public long Size { get; set; }
    
    /// <summary>
    /// Directory containing the file
    /// </summary>
    public string? Directory { get; set; }
    
    /// <summary>
    /// Date when the file was last modified
    /// </summary>
    public DateTime LastModifiedDate { get; set; }
    
    /// <summary>
    /// Date when the file was created
    /// </summary>
    public DateTime CreationDate { get; set; }
    
    /// <summary>
    /// File attributes (Hidden, ReadOnly, System, etc.)
    /// </summary>
    public FileAttributes Attributes { get; set; }
    
    /// <summary>
    /// Indicates if the file is hidden
    /// </summary>
    public bool IsHidden => Attributes.HasFlag(FileAttributes.Hidden);
    
    /// <summary>
    /// Indicates if the file is read-only
    /// </summary>
    public bool IsReadOnly => Attributes.HasFlag(FileAttributes.ReadOnly);

    /// <summary>
    /// Indicates if the file is a system file
    /// </summary>
    public bool IsSystemFile => Attributes.HasFlag(FileAttributes.System);
    
    /// <summary>
    /// Category of the file (Document, Image, Video, etc.)
    /// </summary>
    public FileCategory Category => GetFileCategory(Extension);
    
    public FileItem()
    {
    }

    /// <summary>
    /// Creates a FileItem from a FileInfo object
    /// </summary>
    /// <param name="fileInfo">FileInfo object</param>
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
    
    /// <summary>
    /// Gets file category based on extension
    /// </summary>
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
using FileSorter.Core.Models;

namespace Tests;

public class FileItemTest
{
    private FileInfo _fileInfo;
    private string _fileDirectory = @"F:\projects\FIONA\Tests\testDir\file1.png";

    [SetUp]
    public void Setup()
    {
        _fileInfo = new FileInfo(_fileDirectory);
    }

    [Test]
    public void Test_FileItem_Constructor()
    {   
        // Arrange
        var name = _fileInfo.Name;
        var extension = _fileInfo.Extension;
        var size = _fileInfo.Length;
        var creationDate = _fileInfo.CreationTime;
        var lastModified = _fileInfo.LastWriteTime;
        var directory = _fileInfo.DirectoryName;
        var fullPath = _fileInfo.FullName;
        
        // Act
        var fileItem = new FileItem(_fileInfo);
        
        // Assert
        Assert.That(fileItem.Name, Is.EqualTo(name));
        Assert.That(fileItem.Extension, Is.EqualTo(extension));
        Assert.That(extension, Is.EqualTo(fileItem.Extension));
        Assert.That(fileItem.Size, Is.EqualTo(size));
        Assert.That(fileItem.CreationDate, Is.EqualTo(creationDate));
        Assert.That(fileItem.LastModifiedDate, Is.EqualTo(lastModified));
        Assert.That(fileItem.Directory, Is.EqualTo(directory));
        Assert.That(fileItem.FullPath, Is.EqualTo(fullPath));
    }
}
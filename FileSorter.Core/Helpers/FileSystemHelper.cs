using FileSorter.Core.Models;

namespace FileSorter.Core.Helpers;

public static class FileSystemHelper
{
    public static string[] ExtractFileExtensions(this string[] files)
    {
        return files.Select(file => Path.GetExtension(file).TrimStart('.'))
            .Where(ext => !string.IsNullOrEmpty(ext))
            .ToArray();
    }

    public static DateTime[] ExtractFileDateTimes(this string[] files)
    {
        var dates = files.Select(File.GetLastWriteTime).ToArray();
        return dates;
    }

    public static Dictionary<string, List<string>> SplitDateTimeInMonthAndYear(this string[] files)
    {
        var dates = files.ExtractFileDateTimes();
        var dict = dates.GroupBy(date => date.Year.ToString()).ToDictionary(
            group => group.Key,
            group => group.Select(date => date.Month.ParseNumberWithMonth())
                .Distinct().ToList()
        );
        return dict;
    }

    public static string ParseNumberWithMonth(this int number)
    {
        if (Enum.IsDefined(typeof(Month), number))
            return Enum.GetName((Month)number) ?? throw new InvalidOperationException();
        throw new ArgumentException($"Invalid month number: {number}");
    }
}
use fuqr::{
    generate_qart,
    matrix::Module,
    qart::WeightPixel,
    qr_code::{Mode, Version},
    QrOptions,
};
use image::{ImageBuffer, Rgb};
use ffmpeg::software::scaling::{context::Context, flag::Flags};
use ffmpeg_next as ffmpeg;
use std::io::{self, BufRead};
use std::path::Path;
use std::fs;
use chrono::Local;
use serde::{Deserialize, Serialize};

// ===== Configuration Structure =====
#[derive(Deserialize, Serialize)]
struct Config {
    qart: QArtConfig,
    path: PathConfig,
}

#[derive(Deserialize, Serialize)]
struct QArtConfig {
    use_pattern: bool,
    qr_version: usize,
    x_aspect: usize,
    y_aspect: usize,
    pad_l: usize,
    pad_r: usize,
    content: String,
    threshold: u8,
}

#[derive(Deserialize, Serialize)]
struct PathConfig {
    input_path: String,
    output_path: String,
}

const FPS: u32 = 5;

fn get_default_config() -> Config {
    Config {
        qart: QArtConfig {
            use_pattern: false,
            qr_version: 11,
            x_aspect: 1,
            y_aspect: 1,
            pad_l: 2,
            pad_r: 2,
            content: "Attention Is All You Need.".to_string(),
            threshold: 127,
        },
        path: PathConfig {
            input_path: "example/101798742.jpg".to_string(),
            output_path: "./toqart".to_string(),
        },
    }
}

fn create_config() -> Result<(), Box<dyn std::error::Error>> {
    let config_path = "toqart.toml";
    
    if Path::new(config_path).exists() {
        println!("[WARN] {} already exists. Skipping creation.", config_path);
        return Ok(());
    }
    
    let config = get_default_config();
    let toml_string = toml::to_string_pretty(&config)?;
    
    fs::write(config_path, toml_string)?;
    println!("[OK] Configuration file created: {}", config_path);
    Ok(())
}

fn parse_bool(s: &str) -> Result<bool, String> {
    match s.to_lowercase().as_str() {
        "true" | "1" | "yes" => Ok(true),
        "false" | "0" | "no" => Ok(false),
        _ => Err(format!("Invalid boolean value: {}", s)),
    }
}

fn parse_cli_args(args: &[String]) -> Result<(Option<String>, Config), String> {
    let mut config = get_default_config();
    let mut input_path: Option<String> = None;
    
    let mut i = 1;
    while i < args.len() {
        let arg = &args[i];
        
        if arg.starts_with("--") {
            let key = &arg[2..];
            
            match key {
                "help" => {
                    return Err("help".to_string());
                }
                "create-config" => {
                    return Err("create-config".to_string());
                }
                "use-pattern" => {
                    if i + 1 >= args.len() {
                        return Err("--use-pattern requires a value".to_string());
                    }
                    i += 1;
                    config.qart.use_pattern = parse_bool(&args[i])?;
                }
                "qr-version" => {
                    if i + 1 >= args.len() {
                        return Err("--qr-version requires a value".to_string());
                    }
                    i += 1;
                    config.qart.qr_version = args[i].parse()
                        .map_err(|_| "Invalid qr-version".to_string())?;
                }
                "x-aspect" => {
                    if i + 1 >= args.len() {
                        return Err("--x-aspect requires a value".to_string());
                    }
                    i += 1;
                    config.qart.x_aspect = args[i].parse()
                        .map_err(|_| "Invalid x-aspect".to_string())?;
                }
                "y-aspect" => {
                    if i + 1 >= args.len() {
                        return Err("--y-aspect requires a value".to_string());
                    }
                    i += 1;
                    config.qart.y_aspect = args[i].parse()
                        .map_err(|_| "Invalid y-aspect".to_string())?;
                }
                "pad-l" => {
                    if i + 1 >= args.len() {
                        return Err("--pad-l requires a value".to_string());
                    }
                    i += 1;
                    config.qart.pad_l = args[i].parse()
                        .map_err(|_| "Invalid pad-l".to_string())?;
                }
                "pad-r" => {
                    if i + 1 >= args.len() {
                        return Err("--pad-r requires a value".to_string());
                    }
                    i += 1;
                    config.qart.pad_r = args[i].parse()
                        .map_err(|_| "Invalid pad-r".to_string())?;
                }
                "threshold" => {
                    if i + 1 >= args.len() {
                        return Err("--threshold requires a value".to_string());
                    }
                    i += 1;
                    config.qart.threshold = args[i].parse()
                        .map_err(|_| "Invalid threshold".to_string())?;
                }
                "content" => {
                    if i + 1 >= args.len() {
                        return Err("--content requires a value".to_string());
                    }
                    i += 1;
                    config.qart.content = args[i].clone();
                }
                "output-path" => {
                    if i + 1 >= args.len() {
                        return Err("--output-path requires a value".to_string());
                    }
                    i += 1;
                    config.path.output_path = args[i].clone();
                }
                _ => {
                    return Err(format!("Unknown option: {}", key));
                }
            }
        } else {
            // Positional argument (input file path)
            input_path = Some(arg.clone());
        }
        
        i += 1;
    }
    
    if let Some(path) = input_path {
        config.path.input_path = path;
    }
    
    Ok((None, config))
}

fn show_help() {
    println!("Usage: toqart [OPTIONS] [INPUT_FILE]");
    println!();
    println!("Options:");
    println!("  --use-pattern <BOOL>      Use pattern mode (true/false) [default: false]");
    println!("  --qr-version <NUM>        QR code version (number) [default: 11]");
    println!("  --x-aspect <NUM>          X aspect ratio [default: 1]");
    println!("  --y-aspect <NUM>          Y aspect ratio [default: 1]");
    println!("  --pad-l <NUM>             Left padding [default: 2]");
    println!("  --pad-r <NUM>             Right padding [default: 2]");
    println!("  --content <TEXT>          QR code content [default: Attention Is All You Need.]");
    println!("  --threshold <NUM>         Gray threshold (0-255) used when weighting pixels [default: 127]");
    println!("  --output-path <PATH>      Output directory [default: ./toqart]");
    println!("  --create-config           Create default configuration file");
    println!("  --help                    Show this help message");
    println!();
    println!("Examples:");
    println!("  toqart ./image.png");
    println!("  toqart --qr-version 15 --output-path ./output ./image.jpg");
    println!("  toqart --content 'https://example.com' --pad-l 3 --pad-r 3 ./image.png");
}

fn main() {
    // Check for command line arguments
    let args: Vec<String> = std::env::args().collect();
    
    if args.len() > 1 {
        match args[1].as_str() {
            "--help" => {
                show_help();
                return;
            }
            "--create-config" => {
                if let Err(e) = create_config() {
                    eprintln!("[ERROR] Failed to create config: {}", e);
                    std::process::exit(1);
                }
                return;
            }
            _ => {}
        }
    }
    
    // Try to parse CLI arguments
    let mut config: Config = match parse_cli_args(&args) {
        Ok((_, cfg)) => {
            // If we have CLI arguments for input path, use them
            cfg
        }
        Err(e) => {
            if e == "help" {
                show_help();
                return;
            } else if e == "create-config" {
                if let Err(err) = create_config() {
                    eprintln!("[ERROR] Failed to create config: {}", err);
                    std::process::exit(1);
                }
                return;
            } else {
                eprintln!("[ERROR] {}", e);
                println!("Use --help for usage information");
                std::process::exit(1);
            }
        }
    };
    
    // Try to load and merge with toqart.toml if it exists
    if let Ok(config_content) = fs::read_to_string("toqart.toml") {
        if let Ok(file_config) = toml::from_str::<Config>(&config_content) {
            // Only override with file config if input_path wasn't specified via CLI
            if args.len() == 1 || args.iter().all(|a| !a.starts_with("--") && !a.ends_with(".mp4") && !a.ends_with(".avi") && !a.ends_with(".mov") && !a.ends_with(".mkv")) {
                config = file_config;
                println!("[OK] Configuration loaded from toqart.toml");
            }
        }
    }
    
    // Extract configuration values
    let use_pattern = config.qart.use_pattern;
    let qr_version = config.qart.qr_version;
    let x_aspect = config.qart.x_aspect;
    let y_aspect = config.qart.y_aspect;
    let pad_l = config.qart.pad_l;
    let pad_r = config.qart.pad_r;
    let qr_content = config.qart.content.clone();
    let threshold = config.qart.threshold;
    
    // Calculate dimensions
    let qr_width = qr_version * 4 + 17;
    let img_width = qr_width - (pad_l + pad_r);
    let img_height = ((img_width * y_aspect) / x_aspect) | 1;
    let pad_t = (qr_width - img_height as usize) / 2 - 1;
    let pad_b = (qr_width - img_height as usize) / 2 + 1;
    
    let video_path = if config.path.input_path.is_empty() {
        println!("[INFO] Please enter the video file path:");
        let stdin = io::stdin();
        let mut video_path = String::new();
        stdin.lock().read_line(&mut video_path).expect("Failed to read input");
        video_path.trim().to_string()
    } else {
        config.path.input_path.clone()
    };
    
    let output_dir = &config.path.output_path;
    
    ffmpeg::init().unwrap();
    ffmpeg::log::set_level(ffmpeg::log::Level::Warning);
    fs::create_dir_all(output_dir).expect("Failed to create output directory");
    
    let source_filename = Path::new(&video_path)
        .file_stem()
        .and_then(|name| name.to_str())
        .unwrap_or("output");

    let mut format_context = ffmpeg::format::input(&video_path).unwrap_or_else(|_| {
        panic!("Unable to open video file: {}", video_path);
    });

    let stream = format_context
        .streams()
        .best(ffmpeg::media::Type::Video)
        .ok_or(ffmpeg::Error::StreamNotFound)
        .unwrap();

    let video_stream_index = stream.index();
    
    let decoder_context =
        ffmpeg::codec::context::Context::from_parameters(stream.parameters()).unwrap();
    let mut decoder = decoder_context.decoder().video().unwrap();

    let mut scaler = Context::get(
        decoder.format(),
        decoder.width(),
        decoder.height(),
        ffmpeg::format::Pixel::RGB24,
        img_width as u32,
        img_height as u32,
        Flags::BILINEAR,
    )
    .unwrap();

    let nth_frame = 30 / FPS;
    let mut frame_index = 0;

    let mut receive_and_process = |decoder: &mut ffmpeg::decoder::Video| {
        let mut decoded = ffmpeg::frame::Video::empty();
        while decoder.receive_frame(&mut decoded).is_ok() {
            if frame_index % nth_frame == 0 {
                let mut rgb_frame = ffmpeg::frame::Video::empty();
                scaler.run(&decoded, &mut rgb_frame).unwrap();
                save_qr_frame(
                    rgb_frame.data(0), 
                    rgb_frame.stride(0), 
                    frame_index, 
                    output_dir, 
                    source_filename,
                    use_pattern,
                    qr_version,
                    x_aspect,
                    y_aspect,
                    pad_l,
                    pad_r,
                    pad_t,
                    pad_b,
                    img_width,
                    img_height,
                    qr_width,
                    threshold,
                    &qr_content,
                );
            }
            frame_index += 1;
        }
    };

    for (stream, packet) in format_context.packets() {
        if stream.index() != video_stream_index {
            continue;
        }
        decoder.send_packet(&packet).unwrap();
        receive_and_process(&mut decoder);
    }
    
    decoder.send_eof().unwrap();
    receive_and_process(&mut decoder);
    
    println!("[INFO] Processing completed! QR code images have been saved to: {}", output_dir);
}

fn save_qr_frame(
    frame: &[u8], 
    frame_stride: usize, 
    frame_index: u32, 
    output_dir: &str, 
    source_filename: &str,
    use_pattern: bool,
    qr_version: usize,
    x_aspect: usize,
    y_aspect: usize,
    pad_l: usize,
    pad_r: usize,
    pad_t: usize,
    pad_b: usize,
    img_width: usize,
    img_height: usize,
    qr_width: usize,
    threshold: u8,
    qr_content: &str,
) {
    let mut weights = vec![WeightPixel::new(false, 0); qr_width * qr_width];
    
    for y in 0..img_height {
        for x in 0..img_width {
            let offset = (y * frame_stride) + x * 3;
            let r = frame[offset];
            let g = frame[offset + 1];
            let b = frame[offset + 2];
            let gray = ((r as u16 + g as u16 + b as u16) / 3) as u8;

            let value = if use_pattern {
                if gray < threshold {
                    (x + y) % 6 != 0 || (img_width - 1 - x + y) % 6 != 0
                } else {
                    x % 6 == (y % 6) || x % 6 == (img_width - 1 - y) % 6
                }
            } else {
                gray < threshold
            };

            weights[(QR_WIDTH - 1 - (x + PAD_L)) * QR_WIDTH + (y + PAD_T)] =
                WeightPixel::new(value, 127);
        }
    }

    let qr_options = QrOptions::new()
        .mode(Some(Mode::Byte))
        .min_version(Version(qr_version))
        .strict_version(true)
        .strict_ecl(true);

    let qr_code = generate_qart(qr_content, &qr_options, &weights).unwrap();

    let margin = 2;
    let out_width = qr_width + 2 * margin;
    let img_buf = ImageBuffer::from_fn(out_width as u32, out_width as u32, |rot_x, rot_y| {
        let x = rot_y as usize;
        let y = out_width - 1 - rot_x as usize;

        if x < margin || y < margin || x > out_width - 1 - margin || y > out_width - 1 - margin {
            return Rgb([255, 255, 255]);
        }
        
        let qr_x = (x - margin) as usize;
        let qr_y = (y - margin) as usize;
        let module = qr_code.matrix.get(qr_x, qr_y);

        let on = if (module.has(Module::TIMING) || module.has(Module::ALIGNMENT))
            && (qr_x >= pad_t
                && qr_x <= qr_width - 1 - pad_b
                && qr_y >= pad_l
                && qr_y <= qr_width - 1 - pad_r)
        {
            weights[(qr_y * qr_width) + qr_x].value().clone()
        } else {
            module.has(Module::ON)
        };
        
        if on {
            Rgb([0 as u8, 0, 0])
        } else {
            Rgb([255, 255, 255])
        }
    });

    let now = Local::now();
    
    let filename = format!(
        "{}_{}_{:02}_{:02}_{:02}{:02}_{:02}{:02}_{:04}_{:05}.png",
        now.timestamp(),
        source_filename,
        qr_version,
        if use_pattern { 1 } else { 0 },
        x_aspect,
        y_aspect,
        pad_l,
        pad_r,
        threshold,
        frame_index
    );
    
    let filepath = format!("{}/{}", output_dir, filename);
    img_buf.save(&filepath).unwrap_or_else(|e| {
        eprintln!("[ERROR] Failed to save file {}: {}", filepath, e);
    });
}
